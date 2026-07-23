# gitkit

Toolkit to build Git push workflows with Go

[![tests](https://github.com/fluxcd/gitkit/workflows/tests/badge.svg)](https://github.com/fluxcd/gitkit/actions)
[![report](https://goreportcard.com/badge/github.com/fluxcd/gitkit)](https://goreportcard.com/report/github.com/fluxcd/gitkit)
[![license](https://img.shields.io/github/license/fluxcd/gitkit.svg)](https://github.com/fluxcd/gitkit/blob/master/LICENSE)
[![GoDoc](https://godoc.org/github.com/fluxcd/gitkit?status.svg)](https://godoc.org/github.com/fluxcd/gitkit)
[![release](https://img.shields.io/github/release/fluxcd/gitkit/all.svg)](https://github.com/fluxcd/gitkit/releases)

## Install

```bash
go get github.com/fluxcd/gitkit
```

## Smart HTTP Server

```go
package main

import (
  "log"
  "net/http"
  "github.com/fluxcd/gitkit"
)

func main() {
  // Configure git hooks
  hooks := &gitkit.HookScripts{
    PreReceive: `echo "Hello World!"`,
  }

  // Configure git service
  service := gitkit.New(gitkit.Config{
    Dir:        "/path/to/repos",
    AutoCreate: true,
    AutoHooks:  true,
    Hooks:      hooks,
  })

  // Configure git server. Will create git repos path if it does not exist.
  // If hooks are set, it will also update all repos with new version of hook scripts.
  if err := service.Setup(); err != nil {
    log.Fatal(err)
  }

  http.Handle("/", service)
  
  // Start HTTP server
  if err := http.ListenAndServe(":5000", nil); err != nil {
    log.Fatal(err)
  }
}
```

Run example:

```bash
go run example.go
```

Then try to clone a test repository:

```bash
$ git clone http://localhost:5000/test.git /tmp/test
# Cloning into '/tmp/test'...
# warning: You appear to have cloned an empty repository.
# Checking connectivity... done.

$ cd /tmp/test
$ touch sample

$ git add sample
$ git commit -am "First commit"
# [master (root-commit) fe40c98] First commit
# 1 file changed, 0 insertions(+), 0 deletions(-)
# create mode 100644 sample

$ git push origin master
# Counting objects: 3, done.
# Writing objects: 100% (3/3), 213 bytes | 0 bytes/s, done.
# Total 3 (delta 0), reused 0 (delta 0)
# remote: Hello World! <----------------- pre-receive hook
# To http://localhost:5000/test.git
# * [new branch]      master -> master
```

In the example's console you'll see something like this:

```bash
2016/05/20 20:01:42 request: GET localhost:5000/test.git/info/refs?service=git-upload-pack
2016/05/20 20:01:42 repo-init: creating pre-receive hook for test.git
2016/05/20 20:03:34 request: GET localhost:5000/test.git/info/refs?service=git-receive-pack
2016/05/20 20:03:34 request: POST localhost:5000/test.git/git-receive-pack
```

### Authentication

```go
package main

import (
  "log"
  "net/http"

  "github.com/fluxcd/gitkit"
)

func main() {
  service := gitkit.New(gitkit.Config{
    Dir:        "/path/to/repos",
    AutoCreate: true,
    Auth:       true, // Turned off by default
  })

  // Here's the user-defined authentication function.
  // If return value is false or error is set, user's request will be rejected.
  // You can hook up your database/redis/cache for authentication purposes.
  service.AuthFunc = func(cred gitkit.Credential, req *gitkit.Request) (bool, error) {
    log.Println("user auth request for repo:", cred.Username, cred.Password, req.RepoName)
    return cred.Username == "hello", nil
  }

  http.Handle("/", service)
  http.ListenAndServe(":5000", nil)
}
```

When you start the server and try to clone repo, you'll see password prompt. Two
examples below illustrate both failed and succesful authentication based on the
auth code above.

```bash
$ git clone http://localhost:5000/awesome-sauce.git
# Cloning into 'awesome-sauce'...
# Username for 'http://localhost:5000': foo
# Password for 'http://foo@localhost:5000':
# fatal: Authentication failed for 'http://localhost:5000/awesome-sauce.git/'

$ git clone http://localhost:5000/awesome-sauce.git
# Cloning into 'awesome-sauce'...
# Username for 'http://localhost:5000': hello
# Password for 'http://hello@localhost:5000':
# warning: You appear to have cloned an empty repository.
# Checking connectivity... done.
```

Git also allows using `.netrc` files for authentication purposes. Open your `~/.netrc`
file and add the following line:

```
machine localhost
  login hello
  password world
```

Next time you try clone the same localhost git repo, git wont show password promt.
Keep in mind that the best practice is to use auth tokens instead of plaintext passwords
for authentication. See [Heroku's docs](https://devcenter.heroku.com/articles/authentication#api-token-storage)
for more information.

## SSH server

```go
package main

import (
  "log"
  "github.com/fluxcd/gitkit"
)

// User-defined key lookup function. You can make a call to a database or
// some sort of cache storage (redis/memcached) to speed things up.
// Content is a string containing ssh public key of a user.
func lookupKey(content string) (*gitkit.PublicKey, error) {
  return &gitkit.PublicKey{Id: "12345"}, nil
}

func main() {
  // In the example below you need to specify a full path to a directory that
  // contains all git repositories, and also a directory that has a gitkit specific
  // ssh private and public key pair that used to run ssh server.
  server := gitkit.NewSSH(gitkit.Config{
    Dir:    "/path/to/git/repos",
    KeyDir: "/path/to/gitkit",
  })

  // User-defined key lookup function. All requests will be rejected if this function
  // is not provider. SSH server only accepts key-based authentication.
  server.PublicKeyLookupFunc = lookupKey

  // Specify host and port to run the server on.
  err := server.ListenAndServe(":2222")
  if err != nil {
    log.Fatal(err)
  }
}
```

Example above uses non-standard SSH port 2222, which can't be used for local testing
by default. To make it work you must modify you ssh client configuration file with
the following snippet:

```
$ nano ~/.ssh/config
```

Paste the following:

```
Host localhost
  Port 2222
```

Now that the server is configured, we can fire it up:

```bash
$ go run ssh_server.go
```

First thing you'll need to make sure you have tested the ssh host verification:

```bash
$ ssh git@localhost -p 2222
# The authenticity of host '[localhost]:2222 ([::1]:2222)' can't be established.
# RSA key fingerprint is SHA256:eZwC9VSbVnoHFRY9QKGK3aBSUqkShRF0HxFmQyLmBJs.
# Are you sure you want to continue connecting (yes/no)? yes
# Warning: Permanently added '[localhost]:2222' (RSA) to the list of known hosts.
# Unsupported request type.
# Connection to localhost closed.
```

All good now. `Unsupported request type.` is a succes output since gitkit does not
allow running shell sessions. Assuming you have configured the directory for git
repositories, clone the test repo:

```bash
$ git clone git@localhost:test.git
# Cloning into 'test'...
# remote: Counting objects: 3, done.
# remote: Total 3 (delta 0), reused 0 (delta 0)
# Receiving objects: 100% (3/3), done.
# Checking connectivity... done.
```

Done, you have now ability to run git push/pull. The important stuff in all examples
above is `lookupKey` function. It controls whether user is allowd to authenticate with
ssh or not.

## Receiver

In Git, The first script to run when handling a push from a client is pre-receive. 
It takes a list of references that are being pushed from stdin; if it exits non-zero, 
none of them are accepted. [More on hooks](https://git-scm.com/book/en/v2/Customizing-Git-Git-Hooks).

```go
package main

import (
  "log"
  "os"
  "fmt"

  "github.com/fluxcd/gitkit"
)

// HookInfo contains information about branch, before and after revisions.
// tmpPath is a temporary directory with checked out git tree for the commit.
func receive(hook *gitkit.HookInfo, tmpPath string) error {
  log.Println("Action:", hook.Action)
  log.Println("Ref:", hook.Ref)
  log.Println("Ref name:", hook.RefName)
  log.Println("Old revision:", hook.OldRev)
  log.Println("New revision:", hook.NewRev)

  // Check if push is non fast-forward (force)
  force, err := gitkit.IsForcePush(hook)
  if err != nil {
    return err
  }

  // Reject force push
  if force {
    return fmt.Errorf("non fast-forward pushed are not allowed")
  }

  // Check if branch is being deleted
  if hook.Action == gitkit.BranchDeleteAction {
    fmt.Println("Deleting branch!")
    return nil
  }

  // Getting a commit message is built-in
  message, err := gitkit.ReadCommitMessage(hook.NewRev)
  if err != nil {
    return err
  }
  log.Println("Commit message:", message)

  return nil
}

func main() {
  receiver := gitkit.Receiver{
    MasterOnly:  false,         // if set to true, only pushes to master branch will be allowed
    TmpDir:      "/tmp/gitkit", // directory for temporary git checkouts
    HandlerFunc: receive,       // your handler function
  }

  // Git hook data is provided via STDIN
  if err := receiver.Handle(os.Stdin); err != nil {
    log.Println("Error:", err)
    os.Exit(1) // terminating with non-zero status will cancel push
  }
}
```

To test if receiver works, you will need to add a sample pre-receive hook to any
git repo. With `go run` its easier to debug but final script should be compiled
and will run very fast.

```bash
#!/bin/bash
cat | go run /path/to/your-receiver.go
```

Modify something in the repo, commit the change and push:

```bash
$ git push
# Counting objects: 3, done.
# Delta compression using up to 8 threads.
# Compressing objects: 100% (3/3), done.
# Writing objects: 100% (3/3), 286 bytes | 0 bytes/s, done.
# Total 3 (delta 2), reused 0 (delta 0)
# -------------------------- out receiver output is here ----------------
# remote: 2016/05/24 17:21:37 Ref: refs/heads/master
# remote: 2016/05/24 17:21:37 Old revision: 5ee8d0891d1e5574e427dc16e0908cb9d28551b9
# remote: 2016/05/24 17:21:37 New revision: e13d6b3a27403029fe674e7b911efd468b035a33
# remote: 2016/05/24 17:21:37 Message: Remove stuff
# To git@localhost:dummy-app.git
#    5ee8d08..e13d6b3  master -> master
```

## Extras

### Remove remote: prefix

If your pre-receive script logs anything to STDOUT, the output might look
like this:

```bash
# Writing objects: 100% (3/3), 286 bytes | 0 bytes/s, done.
# Total 3 (delta 2), reused 0 (delta 0)
remote: Sample script output <---- YOUR SCRIPT 
```

There's a simple hack to remove this nasty `remote:` prefix:

```bash
#!/bin/bash
/my/receiver-script | sed -u "s/^/"$'\e[1G\e[K'"/"
```

If you're running on OSX, use `gsed` instead: `brew install gnu-sed`. 

Result:

```bash
# Writing objects: 100% (3/3), 286 bytes | 0 bytes/s, done.
# Total 3 (delta 2), reused 0 (delta 0)
Sample script output
```

## References

- https://git-scm.com/book/en/v2/Git-Internals-Transfer-Protocols

## License

The MIT License

Copyright (c) 2016-2019 Dan Sosedoff, <dan.sosedoff@gmail.com>
