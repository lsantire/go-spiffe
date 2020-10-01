package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/spiffe/go-spiffe/v2/spiffeid"
	"github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig"
	"github.com/spiffe/go-spiffe/v2/svid/x509svid"
	"github.com/spiffe/go-spiffe/v2/workloadapi"
)

const (
	// Worload API socket path
	socketPath     = "unix:///tmp/agent.sock"
	contextTimeout = 3 * time.Second
	readTimeout    = 3 * time.Second
	writeTimeout   = 3 * time.Second
)

var expectedClientID spiffeid.ID
var matcher spiffeid.Matcher

// Global variables initialization
func init() {
	// Allowed SPIFFE ID and it's matcher
	expectedClientID = spiffeid.Must("example.org", "client")
	matcher = spiffeid.MatchID(expectedClientID)
}

func main() {
	ctx, cancel := context.WithTimeout(context.Background(), contextTimeout)
	defer cancel()

	// Set up a `/` resource handler
	http.HandleFunc("/", handler)

	// Create a `workloadapi.X509Source`, it will connect to Workload API using provided socket.
	// If socket path is not defined using `workloadapi.SourceOption`, value from environment variable `SPIFFE_ENDPOINT_SOCKET` is used.
	source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(workloadapi.WithAddr(socketPath)))
	if err != nil {
		log.Fatalf("Unable to create X509Source: %v", err)
	}
	defer source.Close()

	// Create a `tls.Config` to allow mTLS connections, and verify that the presented certificate has any SPIFFE ID
	tlsConfig := tlsconfig.MTLSServerConfig(source, source, tlsconfig.AuthorizeAny())
	server := &http.Server{
		Addr:         ":8443",
		TLSConfig:    tlsConfig,
		ReadTimeout:  readTimeout,
		WriteTimeout: writeTimeout,
	}

	if err := server.ListenAndServeTLS("", ""); err != nil {
		log.Fatalf("Error on serve: %v", err)
	}
}

func handler(w http.ResponseWriter, r *http.Request) {
	log.Println("Request received")

	// Get client's SPIFFE ID from certificate
	clientID, err := x509svid.IDFromCert(r.TLS.PeerCertificates[0])

	// This should never happen, because the TLS config accepts only valid SVIDs as client certificate
	if err != nil {
		log.Printf("An error ocurred while getting the SPIFFE ID from client certificate: %v", err)
		http.Error(w, "Unexpected error", http.StatusInternalServerError)
		return
	}

	// Check whether the clientID matches the expected SPIFFE ID or not
	err = matcher(clientID)
	if err != nil {
		log.Printf("Unauthorized: %v", err)
		http.Error(w, "Unexpected SPIFFE ID", http.StatusUnauthorized)
		return
	}

	log.Printf("OK, expected SPIFFE ID found: %v\n", clientID)
	_, _ = io.WriteString(w, "Success!!!")
}
