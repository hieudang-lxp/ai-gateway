// Command lxp-scan-svc serves the lxp-scan cross-repo analyzer over gRPC.
package main

import (
	"flag"
	"log"
	"net"
	"os"
	"path/filepath"

	pb "lxp-scan-svc/gen/lxpscanpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/reflection"
)

func defaultRoot() string {
	if r := os.Getenv("LXP_SCAN_ROOT"); r != "" {
		return r
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "."
	}
	// The FE workspace lxp-scan analyzes (~9 sibling repos under Leapxpert/FE).
	return filepath.Join(home, "Leapxpert", "FE")
}

func main() {
	log.SetFlags(log.LstdFlags)
	addr := flag.String("addr", "localhost:50051", "listen address")
	root := flag.String("root", defaultRoot(), "default workspace root (per-request overridable; env LXP_SCAN_ROOT)")
	token := flag.String("token", os.Getenv("LXP_SCAN_TOKEN"), "require this bearer token (empty = no auth; env LXP_SCAN_TOKEN)")
	tlsCert := flag.String("tls-cert", "", "TLS certificate file (enables TLS with -tls-key)")
	tlsKey := flag.String("tls-key", "", "TLS private key file")
	flag.Parse()

	var opts []grpc.ServerOption
	if *token != "" {
		opts = append(opts, grpc.UnaryInterceptor(authInterceptor(*token)))
	}
	tlsOn := false
	if *tlsCert != "" || *tlsKey != "" {
		if *tlsCert == "" || *tlsKey == "" {
			log.Fatalf("-tls-cert and -tls-key must be given together")
		}
		creds, err := credentials.NewServerTLSFromFile(*tlsCert, *tlsKey)
		if err != nil {
			log.Fatalf("load TLS: %v", err)
		}
		opts = append(opts, grpc.Creds(creds))
		tlsOn = true
	}

	lis, err := net.Listen("tcp", *addr)
	if err != nil {
		log.Fatalf("listen: %v", err)
	}

	srv := grpc.NewServer(opts...)
	pb.RegisterScanServiceServer(srv, &scanServer{defaultRoot: *root})
	reflection.Register(srv)

	log.Printf("lxp-scan-svc on %s | root=%s | tls=%v | auth=%v", *addr, *root, tlsOn, *token != "")
	if err := srv.Serve(lis); err != nil {
		log.Fatalf("serve: %v", err)
	}
}
