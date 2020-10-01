# HTTP over mTLS

This example shows how two services using HTTP can communicate using mTLS with X509 SVIDs obtained from SPIFFE workload API.

Each service is connecting to the Workload API to fetch its identities. Since this example assumes the SPIRE implementation, it uses the SPIRE default socket path: `/tmp/agent.sock`. 

```go
source, err := workloadapi.NewX509Source(ctx, workloadapi.WithClientOptions(workloadapi.WithAddr(socketPath)))
```

The **HTTP server** uses the [workloadapi.X509Source](https://pkg.go.dev/github.com/spiffe/go-spiffe/v2/workloadapi?tab=doc#X509Source) to create a `tls.Config` for mTLS that authenticates the client certificate and verifies that it has any SPIFFE ID.

The `tls.Config` is used when creating the HTTP server.

```go
tlsConfig := tlsconfig.MTLSServerConfig(source, source, tlsconfig.AuthorizeAny())
server := &http.Server{
    Addr:         ":8443",
    TLSConfig:    tlsConfig,
    ReadTimeout:  readTimeout,
    WriteTimeout: writeTimeout,
}
```

Then, every time a request is received and authenticated, the http handler function gets the client's SPIFFE ID from the presented certificate using [x509svid.IDFromCert](https://pkg.go.dev/github.com/spiffe/go-spiffe/v2@v2.0.0-beta.1/svid/x509svid#IDFromCert) and then checks whether it matches the expected SPIFFE ID or not: `spiffe://examples.org/client`.

```go
expectedClientID = spiffeid.Must("example.org", "client")
matcher = spiffeid.MatchID(expectedClientID)
clientID, err := x509svid.IDFromCert(r.TLS.PeerCertificates[0])
err = matcher(clientID)
```

Depending on the result of the matcher, the server response will be **HTTP 200** if the ids matched, and **HTTP 401** otherwise.
	
On the other side, the **HTTP client** uses the [workloadapi.X509Source](https://pkg.go.dev/github.com/spiffe/go-spiffe/v2/workloadapi?tab=doc#X509Source) to create a `tls.Config` for mTLS that authenticates the server certificate and verifies that it has the SPIFFE ID `spiffe://examples.org/server`. 

```go
serverID := spiffeid.Must("example.org", "server")
tlsConfig := tlsconfig.MTLSClientConfig(source, source, tlsconfig.AuthorizeID(serverID))

client := &http.Client{
    Transport: &http.Transport{
        TLSClientConfig: tlsConfig,
    },
}
```

The [tlsconfig.Authorizer](https://pkg.go.dev/github.com/spiffe/go-spiffe/v2/spiffetls/tlsconfig?tab=doc#Authorizer) is used to authorize the mTLS peer. In this example, the client uses it to authorize the specific SPIFFE ID of the server, and the server uses it to authorize any SPIFFE ID of the client.

That is it! The go-spiffe library fetches and automatically renews the X.509 SVIDs of both workloads from the Workload API provider (i.e. SPIRE).

As soon as the mTLS connection is established, the client sends an HTTP request to the server and gets a response.

## Building
Build the client workload:
```bash
cd examples/spiffe-http/client
go build
```

Build the server workload:
```bash
cd examples/spiffe-http/server
go build
```

## Running
This example assumes the following preconditions:
- There is a SPIRE server and agent up and running.
- There is a Unix workload attestor configured.
- The trust domain is `example.org`
- The agent SPIFFE ID is `spiffe://example.org/host`.
- There is a `server-workload` and `client-workload` user in the system.

Check the [SPIRE101 Walkthrough](https://github.com/spiffe/spire/blob/master/doc/SPIRE101.md) for more information on how to set up all this preconditions.

### 1. Create the registration entries
Create the registration entries for the client and server workloads:

Server:
```bash
./spire-server entry create -spiffeID spiffe://example.org/server \
                            -parentID spiffe://example.org/host \
                            -selector unix:user:server-workload
```

Client: 
```bash
./spire-server entry create -spiffeID spiffe://example.org/client \
                            -parentID spiffe://example.org/host \
                            -selector unix:user:client-workload
```

### 2. Start the server
Start the server with the `server-workload` user:
```bash
sudo -u server-workload ./server
```

### 3. Run the client
Run the client with the `client-workload` user:
```bash
sudo -u client-workload ./client
```

The server should display a log `Request received` and then `OK, expected SPIFFE ID found: spiffe://examples.org/client`.
The client should display a log `Response: status=200 OK, body=Success!!!`

If the client encounters a peer with a different SPIFFE ID, the TLS handshake will be aborted and the connection will fail.

```
sudo -u client-workload ./server

TLS handshake error from 127.0.0.1:52802: remote error: tls: bad certificate
```

And client log shows

```
Error connecting to "https://localhost:8443/": Get https://localhost:8443/: unexpected ID "spiffe://example.org/client"
```


If the server encounters a peer with a different SPIFFE ID, the TLS handshake won't fail, but the response will be **HTTP 401 - Unauthorized**

```
sudo -u server-workload ./client

Response: status=401 Unauthorized, body=Unexpected SPIFFE ID
```

And server log shows

```
Unauthorized: unexpected ID "spiffe://example.org/server"
```
