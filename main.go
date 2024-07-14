package main

import (
	"bufio"
	"crypto/rand"
	"fmt"
	"io"
	"log"
	mrand "math/rand"
	"net/http"

	"github.com/libp2p/go-libp2p"
	"github.com/libp2p/go-libp2p/core/crypto"
	"github.com/libp2p/go-libp2p/core/host"
	"github.com/libp2p/go-libp2p/core/network"

	"github.com/gin-gonic/gin"
	golog "github.com/ipfs/go-log/v2"
	ma "github.com/multiformats/go-multiaddr"
)

// album represents data about a record album.
type album struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Artist string  `json:"artist"`
	Price  float64 `json:"price"`
}

// albums slice to seed record album data.
var albums = []album{
	{ID: "1", Title: "Blue Train", Artist: "John Coltrane", Price: 56.99},
	{ID: "2", Title: "Jeru", Artist: "Gerry Mulligan", Price: 17.99},
	{ID: "3", Title: "Sarah Vaughan and Clifford Brown", Artist: "Sarah Vaughan", Price: 39.99},
}

const (
	WEB_PORT = 8080
	P2P_PORT = 8081
)

var ha host.Host

func main() {
	gin.SetMode(gin.ReleaseMode)

	// LibP2P code uses golog to log messages. They log with different
	// string IDs (i.e. "swarm"). We can control the verbosity level for
	// all loggers with:
	golog.SetAllLoggers(golog.LevelInfo) // Change to INFO for extra info

	// Make a host that listens on the given multiaddress
	var err error
	ha, err = makeBasicHost(P2P_PORT, false, 0)
	if err != nil {
		log.Fatal(err)
	}

	// Set a function as a stream handler.
	// This function is called when a peer connects and opens a
	// stream to this peer on the /echo/1.0.0 protocol
	ha.SetStreamHandler("/echo/1.0.0", func(s network.Stream) {
		log.Println("Got a new stream!")
		if err := doEcho(s); err != nil {
			log.Println(err)
			s.Reset()
		} else {
			s.Close()
		}
	})

	// Start the HTTP server using Gin in a goroutine
	go func() {
		router := gin.Default()
		router.GET("/connection-info", getConnectionInfo)
		router.GET("/albums", getAlbums)
		if err := router.Run(fmt.Sprintf(":%d", WEB_PORT)); err != nil {
			log.Fatal(err)
		}
	}()

	// Keep the main goroutine alive
	select {}
}

// getConnectionInfo returns the URI with the hash
func getConnectionInfo(c *gin.Context) {
	fullAddr := getHostAddress(ha)
	c.IndentedJSON(http.StatusOK, gin.H{"connection_info": fullAddr})
}

// makeBasicHost creates a LibP2P host with a random peer ID listening on the
// given port
func makeBasicHost(listenPort int, insecure bool, randseed int64) (host.Host, error) {
	// If the seed is zero, use real cryptographic randomness. Otherwise, use a
	// deterministic randomness source to make generated keys stay the same
	// across multiple runs
	var r io.Reader
	if randseed == 0 {
		r = rand.Reader
	} else {
		r = mrand.New(mrand.NewSource(randseed))
	}

	// Generate a key pair for this host. We will use it to obtain a valid host ID.
	priv, _, err := crypto.GenerateKeyPairWithReader(crypto.Ed25519, 2048, r)
	if err != nil {
		return nil, err
	}

	opts := []libp2p.Option{
		libp2p.ListenAddrStrings(fmt.Sprintf("/ip4/0.0.0.0/tcp/%d", listenPort)),
		libp2p.Identity(priv),
	}

	if insecure {
		opts = append(opts, libp2p.NoSecurity)
	}

	bhost, err := libp2p.New(opts...)
	if err != nil {
		return nil, err
	}

	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", bhost.ID().String()))

	addrs := bhost.Addrs()
	if len(addrs) == 0 {
		return nil, fmt.Errorf("no addresses")
	}
	addr := addrs[0]
	fullAddr := addr.Encapsulate(hostAddr)
	log.Printf("I am %s\n", fullAddr)
	return bhost, nil
}

// getHostAddress returns the full multiaddress of the host
func getHostAddress(h host.Host) string {
	hostAddr, _ := ma.NewMultiaddr(fmt.Sprintf("/p2p/%s", h.ID().String()))
	addrs := h.Addrs()
	if len(addrs) == 0 {
		return "no addresses"
	}
	addr := addrs[0]
	fullAddr := addr.Encapsulate(hostAddr)
	return fullAddr.String()
}

// doEcho reads a line of data from a stream and writes it back
func doEcho(s network.Stream) error {
	buf := bufio.NewReader(s)
	str, err := buf.ReadString('\n')
	if err != nil {
		return err
	}

	log.Printf("read: %s", str)
	_, err = s.Write([]byte(str))
	return err
}

// getAlbums responds with the list of all albums as JSON.
func getAlbums(c *gin.Context) {
	c.IndentedJSON(http.StatusOK, albums)
}
