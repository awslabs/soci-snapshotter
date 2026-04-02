// Package main implements a minimal Docker registry token server for integration testing.
// It validates Basic auth credentials against a password file and issues JWTs
// compatible with Docker Distribution's token verification.
package main

import (
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

func main() {
	certFile := flag.String("cert", "", "TLS certificate file")
	keyFile := flag.String("key", "", "TLS key file")
	signingKeyFile := flag.String("signing-key", "", "Token signing key (PEM, RSA)")
	passwordFile := flag.String("password-file", "", "File containing accepted password")
	addr := flag.String("addr", ":5001", "Listen address")
	issuer := flag.String("issuer", "test-issuer", "Token issuer")
	flag.Parse()

	signingKeyPEM, err := os.ReadFile(*signingKeyFile)
	if err != nil {
		log.Fatalf("failed to read signing key: %v", err)
	}
	block, _ := pem.Decode(signingKeyPEM)
	signingKeyRaw, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		log.Fatalf("failed to parse signing key: %v", err)
	}
	signingKey := signingKeyRaw.(*rsa.PrivateKey)

	http.HandleFunc("/token", func(w http.ResponseWriter, r *http.Request) {
		user, pass, ok := r.BasicAuth()
		if !ok || user == "" {
			w.Header().Set("WWW-Authenticate", `Basic realm="token"`)
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		currentPass, err := os.ReadFile(*passwordFile)
		if err != nil {
			http.Error(w, "internal error", http.StatusInternalServerError)
			return
		}
		if strings.TrimSpace(string(currentPass)) != pass {
			w.Header().Set("WWW-Authenticate", `Basic realm="token"`)
			http.Error(w, "bad credentials", http.StatusUnauthorized)
			return
		}

		service := r.URL.Query().Get("service")
		scope := r.URL.Query().Get("scope")

		token, err := makeToken(signingKey, *issuer, service, user, scope)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"token":      token,
			"expires_in": 3600,
			"issued_at":  time.Now().UTC().Format(time.RFC3339),
		})
	})

	log.Printf("token server listening on %s", *addr)
	if err := http.ListenAndServeTLS(*addr, *certFile, *keyFile, nil); err != nil {
		log.Fatal(err)
	}
}

func makeToken(key *rsa.PrivateKey, issuer, service, subject, scope string) (string, error) {
	now := time.Now().UTC()

	var access []map[string]interface{}
	if scope != "" {
		parts := strings.SplitN(scope, ":", 3)
		if len(parts) == 3 {
			access = append(access, map[string]interface{}{
				"type":    parts[0],
				"name":    parts[1],
				"actions": strings.Split(parts[2], ","),
			})
		}
	}

	header := map[string]interface{}{
		"typ": "JWT",
		"alg": "RS256",
		"kid": keyID(&key.PublicKey),
	}
	claims := map[string]interface{}{
		"iss":    issuer,
		"sub":    subject,
		"aud":    service,
		"exp":    now.Add(time.Hour).Unix(),
		"nbf":    now.Add(-time.Second).Unix(),
		"iat":    now.Unix(),
		"jti":    fmt.Sprintf("%d", now.UnixNano()),
		"access": access,
	}

	hdr, err := encodeJSON(header)
	if err != nil {
		return "", err
	}
	clm, err := encodeJSON(claims)
	if err != nil {
		return "", err
	}

	payload := hdr + "." + clm
	hash := sha256.Sum256([]byte(payload))
	sig, err := rsa.SignPKCS1v15(rand.Reader, key, crypto.SHA256, hash[:])
	if err != nil {
		return "", err
	}

	return payload + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func encodeJSON(v interface{}) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// keyID generates a Key ID matching Docker Distribution's libtrust format:
// base32(SHA256(DER(pubkey))[:30]) formatted as XXXX:XXXX:XXXX:...
func keyID(pub *rsa.PublicKey) string {
	der, _ := x509.MarshalPKIXPublicKey(pub)
	hash := sha256.Sum256(der)
	s := strings.TrimRight(base32.StdEncoding.EncodeToString(hash[:30]), "=")
	var groups []string
	for i := 0; i < len(s); i += 4 {
		end := i + 4
		if end > len(s) {
			end = len(s)
		}
		groups = append(groups, s[i:end])
	}
	return strings.Join(groups, ":")
}
