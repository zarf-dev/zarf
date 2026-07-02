package gitkit

import (
	"bytes"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.org/x/crypto/ssh"
)

var (
	ErrAlreadyStarted = errors.New("server has already been started")
	ErrNoListener     = errors.New("cannot call Serve() before Listen()")
)

type PublicKey struct {
	Id          string
	Name        string
	Fingerprint string
	Content     string
}

type SSH struct {
	listener net.Listener

	sshConfig *ssh.ServerConfig
	gitConfig *Config
	// Timeout, if set will close the connection after the given duration
	Timeout *time.Duration
	// DisableConnReuse, if true will disable a reuse of ssh connection in a later session.
	DisableConnReuse bool
	// DisableSimultaneousConns, if true will disable simultaneous conns from the same host.
	DisableSimultaneousConns bool
	PublicKeyLookupFunc      func(string) (*PublicKey, error)
}

func NewSSH(config Config) *SSH {
	s := &SSH{gitConfig: &config}

	// Use PATH if full path is not specified
	if s.gitConfig.GitPath == "" {
		s.gitConfig.GitPath = "git"
	}
	return s
}

// Sets the sshConfig of SSH to the given ssh.ServerConfig
func (s *SSH) SetSSHConfig(config *ssh.ServerConfig) {
	s.sshConfig = config
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil || os.IsExist(err)
}

func cleanCommand(cmd string) string {
	i := strings.Index(cmd, "git")
	if i == -1 {
		return cmd
	}
	return cmd[i:]
}

func execCommandBytes(cmdname string, args ...string) ([]byte, []byte, error) {
	bufOut := new(bytes.Buffer)
	bufErr := new(bytes.Buffer)

	cmd := exec.Command(cmdname, args...)
	cmd.Stdout = bufOut
	cmd.Stderr = bufErr

	err := cmd.Run()
	return bufOut.Bytes(), bufErr.Bytes(), err
}

func execCommand(cmdname string, args ...string) (string, string, error) {
	bufOut, bufErr, err := execCommandBytes(cmdname, args...)
	return string(bufOut), string(bufErr), err
}

func (s *SSH) handleConnection(keyID string, chans <-chan ssh.NewChannel, sConn *ssh.ServerConn) {
	for newChan := range chans {
		if newChan.ChannelType() != "session" {
			newChan.Reject(ssh.UnknownChannelType, "unknown channel type")
			continue
		}

		ch, reqs, err := newChan.Accept()
		if err != nil {
			log.Printf("error accepting channel: %v", err)
			continue
		}

		go func(in <-chan *ssh.Request) {
			defer ch.Close()

			defer func() {
				if s.DisableConnReuse {
					err := sConn.Close()
					if err != nil {
						log.Println("err while closing:", err)
					}
				}
				if s.DisableSimultaneousConns {
					host, _ := getHost(sConn.RemoteAddr().String())
					mux.Lock()
					defer mux.Unlock()
					log.Println("disable simultaneous conns")
					for i, connHost := range connHosts {
						if host == connHost {
							connHosts[i] = connHosts[len(connHosts)-1]
							connHosts = connHosts[:len(connHosts)-1]
						}
					}
				}
			}()

			for req := range in {
				payload := cleanCommand(string(req.Payload))

				switch req.Type {
				case "env":
					log.Printf("ssh: incoming env request: %s\n", payload)

					args := strings.Split(strings.Replace(payload, "\x00", "", -1), "\v")
					if len(args) != 2 {
						log.Printf("env: invalid env arguments: '%#v'", args)
						continue
					}

					args[0] = strings.TrimLeft(args[0], "\x04")
					if len(args[0]) == 0 {
						log.Printf("env: invalid key from payload: %s", payload)
						continue
					}

					_, _, err := execCommandBytes("env", args[0]+"="+args[1])
					if err != nil {
						log.Printf("env: %v", err)
						return
					}
				case "exec":
					log.Printf("ssh: incoming exec request: %s\n", payload)

					cmdName := strings.TrimLeft(payload, "'()")
					log.Printf("ssh: payload '%v'", cmdName)

					if strings.HasPrefix(cmdName, "\x00") {
						cmdName = strings.Replace(cmdName, "\x00", "", -1)[1:]
					}

					gitcmd, err := ParseGitCommand(cmdName)
					if err != nil {
						log.Println("ssh: error parsing command:", err)
						ch.Write([]byte("Invalid command.\r\n"))
						return
					}

					if !repoExists(filepath.Join(s.gitConfig.Dir, gitcmd.Repo)) && s.gitConfig.AutoCreate == true {
						err := initRepo(gitcmd.Repo, s.gitConfig)
						if err != nil {
							logError("repo-init", err)
							return
						}
					}

					// Simulates servers that short-circuit the connection
					// when the user does not have permissions to finish
					// the operation at hand.
					//
					// During a git push, this leads to an 'EOF' error.
					if gitcmd.Command == "git-receive-pack" && s.gitConfig.ReadOnly {
						sConn.Close()
						break
					}

					cmd := exec.Command(gitcmd.Command, gitcmd.Repo)
					cmd.Dir = s.gitConfig.Dir
					cmd.Env = append(os.Environ(), "GITKIT_KEY="+keyID)
					// cmd.Env = append(os.Environ(), "SSH_ORIGINAL_COMMAND="+cmdName)

					stdout, err := cmd.StdoutPipe()
					if err != nil {
						log.Printf("ssh: cant open stdout pipe: %v", err)
						return
					}

					stderr, err := cmd.StderrPipe()
					if err != nil {
						log.Printf("ssh: cant open stderr pipe: %v", err)
						return
					}

					input, err := cmd.StdinPipe()
					if err != nil {
						log.Printf("ssh: cant open stdin pipe: %v", err)
						return
					}

					if err = cmd.Start(); err != nil {
						log.Printf("ssh: start error: %v", err)
						return
					}

					req.Reply(true, nil)
					go io.Copy(input, ch)
					io.Copy(ch, stdout)
					io.Copy(ch.Stderr(), stderr)

					if err = cmd.Wait(); err != nil {
						log.Printf("ssh: command failed: %v", err)
						return
					}

					ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
					return
				default:
					ch.Write([]byte("Unsupported request type.\r\n"))
					log.Println("ssh: unsupported req type:", req.Type)
					return
				}
				if s.DisableConnReuse {
					log.Println("dispose connection")
					break
				}
			}
		}(reqs)
	}
}

func (s *SSH) createServerKey() error {
	if err := os.MkdirAll(s.gitConfig.KeyDir, os.ModePerm); err != nil {
		return err
	}

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}

	privateKeyFile, err := os.Create(s.gitConfig.KeyPath())
	if err != nil {
		return err
	}

	if err := os.Chmod(s.gitConfig.KeyPath(), 0600); err != nil {
		return err
	}
	defer privateKeyFile.Close()
	if err != nil {
		return err
	}
	privateKeyPEM := &pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)}
	if err := pem.Encode(privateKeyFile, privateKeyPEM); err != nil {
		return err
	}

	pubKeyPath := s.gitConfig.KeyPath() + ".pub"
	pub, err := ssh.NewPublicKey(&privateKey.PublicKey)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(pubKeyPath, ssh.MarshalAuthorizedKey(pub), 0644)
}

func (s *SSH) setup() error {
	var config *ssh.ServerConfig
	if s.sshConfig != nil {
		config = s.sshConfig
	} else {
		config = &ssh.ServerConfig{}
	}
	config.ServerVersion = fmt.Sprintf("SSH-2.0-gitkit %s", Version)

	if s.gitConfig.KeyDir == "" {
		return fmt.Errorf("key directory is not provided")
	}

	if !s.gitConfig.Auth {
		config.NoClientAuth = true
	} else {
		if s.PublicKeyLookupFunc == nil {
			return fmt.Errorf("public key lookup func is not provided")
		}

		config.PublicKeyCallback = func(conn ssh.ConnMetadata, key ssh.PublicKey) (*ssh.Permissions, error) {
			pkey, err := s.PublicKeyLookupFunc(strings.TrimSpace(string(ssh.MarshalAuthorizedKey(key))))
			if err != nil {
				return nil, err
			}

			if pkey == nil {
				return nil, fmt.Errorf("auth handler did not return a key")
			}

			return &ssh.Permissions{Extensions: map[string]string{"key-id": pkey.Id}}, nil
		}
	}

	keypath := s.gitConfig.KeyPath()
	if !fileExists(keypath) {
		if err := s.createServerKey(); err != nil {
			return err
		}
	}

	privateBytes, err := ioutil.ReadFile(keypath)
	if err != nil {
		return err
	}

	private, err := ssh.ParsePrivateKey(privateBytes)
	if err != nil {
		return err
	}

	config.AddHostKey(private)
	s.sshConfig = config
	return nil
}

func (s *SSH) Listen(bind string) error {
	if s.listener != nil {
		return ErrAlreadyStarted
	}

	if err := s.setup(); err != nil {
		return err
	}

	if err := s.gitConfig.Setup(); err != nil {
		return err
	}

	var err error
	s.listener, err = net.Listen("tcp", bind)
	if err != nil {
		return err
	}

	return nil
}

var mux sync.Mutex
var connHosts []string

func getHost(addr string) (string, error) {
	if !strings.HasPrefix(addr, "ssh://") {
		addr = "ssh://" + addr
	}
	u, err := url.Parse(addr)
	if err != nil {
		return "", err
	}
	host, _, err := net.SplitHostPort(u.Host)
	if err != nil {
		return "", err
	}
	return host, nil
}

func (s *SSH) Serve() error {
	if s.listener == nil {
		return ErrNoListener
	}

	for {
		// wait for connection or Stop()
		conn, err := s.listener.Accept()
		if err != nil {
			return err
		}

		if s.DisableSimultaneousConns {
			mux.Lock()
			defer mux.Unlock()
			host, _ := getHost(conn.RemoteAddr().String())
			var matched bool
			for _, connHost := range connHosts {
				if host == connHost {
					log.Println("can't have two multiple simultaneous connections from the same client")
					err := conn.Close()
					if err != nil {
						log.Println("err while closing:", err)
					}
					matched = true
					break
				}
			}
			if !matched {
				connHosts = append(connHosts, host)
			} else {
				continue
			}
		}

		if s.Timeout != nil {
			go func(conn net.Conn) {
				time.Sleep(*s.Timeout)
				conn.Close()
			}(conn)
		}

		go func() {
			log.Printf("ssh: handshaking for %s", conn.RemoteAddr())

			sConn, chans, reqs, err := ssh.NewServerConn(conn, s.sshConfig)
			if err != nil {
				if err == io.EOF {
					log.Printf("ssh: handshaking was terminated: %v", err)
				} else {
					log.Printf("ssh: error on handshaking: %v", err)
				}
				return
			}

			log.Printf("ssh: connection from %s (%s)", sConn.RemoteAddr(), sConn.ClientVersion())

			if s.gitConfig.Auth && s.gitConfig.GitUser != "" && sConn.User() != s.gitConfig.GitUser {
				sConn.Close()
				return
			}

			keyId := ""
			if sConn.Permissions != nil {
				keyId = sConn.Permissions.Extensions["key-id"]
			}

			go ssh.DiscardRequests(reqs)
			go s.handleConnection(keyId, chans, sConn)
		}()
	}
}

func (s *SSH) ListenAndServe(bind string) error {
	if err := s.Listen(bind); err != nil {
		return err
	}
	return s.Serve()
}

// Stop stops the server if it has been started, otherwise it is a no-op.
func (s *SSH) Stop() error {
	if s.listener == nil {
		return nil
	}
	defer func() {
		s.listener = nil
	}()

	return s.listener.Close()
}

// Address returns the network address of the listener. This is in
// particular useful when binding to :0 to get a free port assigned by
// the OS.
func (s *SSH) Address() string {
	if s.listener != nil {
		return s.listener.Addr().String()
	}
	return ""
}
