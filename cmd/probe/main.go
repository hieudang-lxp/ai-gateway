// Command probe is a tiny typed gRPC client for smoke-testing lxp-scan-svc.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	pb "lxp-scan-svc/gen/lxpscanpb"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"
)

// bearer sends "authorization: Bearer <token>" on every RPC.
type bearer struct{ token string }

func (b bearer) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + b.token}, nil
}
func (b bearer) RequireTransportSecurity() bool { return false }

func main() {
	addr := flag.String("addr", "localhost:50051", "server address")
	root := flag.String("root", "", "workspace root override")
	token := flag.String("token", "", "bearer token")
	useTLS := flag.Bool("tls", false, "use TLS")
	ca := flag.String("ca", "", "CA/cert file for TLS")
	flag.Parse()

	opts := []grpc.DialOption{}
	if *useTLS {
		creds, err := credentials.NewClientTLSFromFile(*ca, "")
		if err != nil {
			log.Fatalf("load CA: %v", err)
		}
		opts = append(opts, grpc.WithTransportCredentials(creds))
	} else {
		opts = append(opts, grpc.WithTransportCredentials(insecure.NewCredentials()))
	}
	if *token != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(bearer{*token}))
	}

	conn, err := grpc.NewClient(*addr, opts...)
	if err != nil {
		log.Fatalf("dial: %v", err)
	}
	defer conn.Close()
	c := pb.NewScanServiceClient(conn)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	imp, err := c.Impact(ctx, &pb.ImpactRequest{Symbol: "Button", From: "fake-lib", Root: *root})
	if err != nil {
		log.Fatalf("Impact: %v", err)
	}
	fmt.Printf("Impact: %d site(s)\n", len(imp.GetSites()))
	for _, s := range imp.GetSites() {
		fmt.Printf("  %s/%s:%d  jsx×%d  from %s  props=%v\n",
			s.GetRepo(), s.GetFile(), s.GetLine(), s.GetJsxUses(), s.GetSource(), s.GetJsxProps())
	}

	dr, err := c.Drift(ctx, &pb.DriftRequest{Root: *root})
	if err != nil {
		log.Fatalf("Drift: %v", err)
	}
	fmt.Printf("Drift: %d row(s)\n", len(dr.GetRows()))
	for _, r := range dr.GetRows() {
		fmt.Printf("  %-26s level=%s versions=%d\n", r.GetPkg(), r.GetLevel(), len(r.GetVersions()))
	}

	cl, err := c.Clones(ctx, &pb.ClonesRequest{Symbol: "validateEmail", Root: *root})
	if err != nil {
		log.Fatalf("Clones: %v", err)
	}
	fmt.Printf("Clones: %d cluster(s)\n", len(cl.GetClusters()))
	for _, cc := range cl.GetClusters() {
		names := make([]string, 0, len(cc.GetMembers()))
		for _, m := range cc.GetMembers() {
			names = append(names, m.GetName())
		}
		fmt.Printf("  %d members: %v (tokens=%d)\n", len(cc.GetMembers()), names, cc.GetTokenCount())
	}

	cx, err := c.Context(ctx, &pb.ContextRequest{Symbol: "Button", Sites: 2, Root: *root})
	if err != nil {
		log.Fatalf("Context: %v", err)
	}
	defName := "<none>"
	if cx.GetDefinition() != nil {
		defName = fmt.Sprintf("%s/%s:%d", cx.GetDefinition().GetRepo(), cx.GetDefinition().GetFile(), cx.GetDefinition().GetLine())
	}
	fmt.Printf("Context %q: sites=%d props=%d def=%s excerpts=%d\n",
		cx.GetSymbol(), cx.GetTotalSites(), len(cx.GetPropCounts()), defName, len(cx.GetExcerpts()))

	// Error path.
	if _, err := c.Impact(ctx, &pb.ImpactRequest{Root: *root}); err != nil {
		fmt.Printf("Impact(no symbol) correctly rejected: %v\n", err)
	} else {
		log.Fatalf("expected error for missing symbol")
	}
}
