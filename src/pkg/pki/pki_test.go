// SPDX-License-Identifier: Apache-2.0
// SPDX-FileCopyrightText: 2021-Present The Zarf Authors
package pki

import (
	"crypto/x509"
	"testing"
	"time"
)

func TestGenerateCA(t *testing.T) {
	type args struct {
		validFor time.Duration
	}
	tests := []struct {
		name         string
		args         args
		wantErr      bool
		wantNotAfter time.Time
	}{
		{
			name: "Valid duration",
			args: args{
				validFor: time.Hour * 24 * 375, // 13 months
			},
			wantErr:      false,
			wantNotAfter: time.Now().Add(time.Hour * 24 * 375),
		},
		{
			name: "Zero duration",
			args: args{
				validFor: 0,
			},
			wantErr:      true,
			wantNotAfter: time.Time{},
		},
		{
			name: "Negative duration",
			args: args{
				validFor: -time.Hour,
			},
			wantErr:      true,
			wantNotAfter: time.Time{},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ca, privKey, err := generateCA(tt.args.validFor)
			if (err != nil) != tt.wantErr {
				t.Errorf("generateCA() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr {
				// Validate private key is not nil and has a public key
				if privKey == nil || privKey.Public() == nil {
					t.Errorf("generateCA() private key is invalid")
				}
				// Allow a small window for time.Now() difference between test setup and actual function call
				if ca.NotAfter.After(tt.wantNotAfter.Add(time.Minute)) || ca.NotAfter.Before(tt.wantNotAfter.Add(-time.Minute)) {
					t.Errorf("generateCA() NotAfter = %v, wantNotAfter %v", ca.NotAfter, tt.wantNotAfter)
				}
			}
		})
	}
}

func Test_generateCert(t *testing.T) {
	type args struct {
		host     string
		ca       *x509.Certificate
		validFor time.Duration
		dnsNames []string
	}

	tests := []struct {
		name         string
		args         args
		wantErr      bool
		wantDNSNames []string
	}{
		{
			name: "Valid certificate generation",
			args: args{
				host:     "localhost",
				ca:       &x509.Certificate{}, // Simplified for example, in practice, use a valid CA certificate
				validFor: time.Hour * 24 * 375,
				dnsNames: []string{"localhost.localdomain"},
			},
			wantErr:      false,
			wantDNSNames: []string{"localhost", "localhost.localdomain"},
		},
		{
			name: "Invalid duration",
			args: args{
				host:     "localhost",
				ca:       &x509.Certificate{}, // Simplified for example, in practice, use a valid CA certificate
				validFor: -time.Hour,
				dnsNames: []string{"localhost.localdomain"},
			},
			wantErr: true,
		},
		{
			name: "Empty host",
			args: args{
				host:     "",
				ca:       &x509.Certificate{}, // Simplified for example, in practice, use a valid CA certificate
				validFor: time.Hour * 24 * 375,
				dnsNames: []string{"localhost.localdomain"},
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cakey, err := newPrivateKey()
			if err != nil {
				t.Fatalf("newPrivateKey() error = %v", err)
			}
			cert, _, err := generateCert(tt.args.host, tt.args.ca, cakey, tt.args.validFor, tt.args.dnsNames...)
			if (err != nil) != tt.wantErr {
				t.Errorf("generateCert() error = %v, wantErr %v", err, tt.wantErr)
			} else if !tt.wantErr {
				// Validate DNS Names
				if !equalSlices(cert.DNSNames, tt.wantDNSNames) {
					t.Errorf("generateCert() DNSNames = %v, want %v", cert.DNSNames, tt.wantDNSNames)
				}
			}
		})
	}
}

// Helper function to compare two slices of strings
func equalSlices[T comparable](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	for i, v := range a {
		if v != b[i] {
			return false
		}
	}
	return true
}

func TestGeneratePKI(t *testing.T) {
	type args struct {
		host     string
		dnsNames []string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Valid host and DNS names",
			args: args{
				host:     "example.com",
				dnsNames: []string{"www.example.com", "mail.example.com"},
			},
			wantErr: false,
		},
		{
			name: "No DNS names",
			args: args{
				host:     "example.com",
				dnsNames: []string{},
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := GeneratePKI(tt.args.host, tt.args.dnsNames...)
			if (got.CA == nil || got.Cert == nil || got.Key == nil) != tt.wantErr {
				t.Errorf("GeneratePKI() error = %v, wantErr %v", got, tt.wantErr)
			} else if !tt.wantErr {
				// Additional validity checks can be performed here, such as verifying the certificate chain
				// This is a simplified check assuming the presence of CA, Cert, and Key indicates success
				if len(got.CA) == 0 || len(got.Cert) == 0 || len(got.Key) == 0 {
					t.Errorf("GeneratePKI() returned empty results, want valid PKI components")
				}
			}
		})
	}
}
