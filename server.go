package main

import (
	"context"

	pb "lxp-scan-svc/gen/lxpscanpb"
	"lxp-scan-svc/internal/scan"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

// scanServer implements ScanService by delegating to the lxp-scan Rust library
// and mapping its JSON payloads onto typed proto responses.
type scanServer struct {
	pb.UnimplementedScanServiceServer
	defaultRoot string
}

func (s *scanServer) rootFor(root string) string {
	if root != "" {
		return root
	}
	return s.defaultRoot
}

func internalErr(err error) error {
	return status.Errorf(codes.Internal, "scan: %v", err)
}

func (s *scanServer) Impact(_ context.Context, req *pb.ImpactRequest) (*pb.ImpactResponse, error) {
	if req.GetSymbol() == "" {
		return nil, status.Error(codes.InvalidArgument, "symbol is required")
	}
	sites, err := scan.Impact(s.rootFor(req.GetRoot()), req.GetSymbol(), req.GetFrom())
	if err != nil {
		return nil, internalErr(err)
	}
	out := &pb.ImpactResponse{Sites: make([]*pb.ImpactSite, 0, len(sites))}
	for _, h := range sites {
		out.Sites = append(out.Sites, &pb.ImpactSite{
			Repo:     h.Repo,
			File:     h.File,
			Line:     h.Line,
			Source:   h.Source,
			Refs:     h.Refs,
			JsxUses:  h.JsxUses,
			JsxProps: h.JsxProps,
			JsxLines: h.JsxLines,
		})
	}
	return out, nil
}

func (s *scanServer) Drift(_ context.Context, req *pb.DriftRequest) (*pb.DriftResponse, error) {
	rows, err := scan.Drift(s.rootFor(req.GetRoot()))
	if err != nil {
		return nil, internalErr(err)
	}
	out := &pb.DriftResponse{Rows: make([]*pb.DriftRow, 0, len(rows))}
	for _, r := range rows {
		versions := make(map[string]*pb.VersionCell, len(r.Versions))
		for repo, cell := range r.Versions {
			versions[repo] = &pb.VersionCell{Version: cell.Version, Source: cell.Source}
		}
		out.Rows = append(out.Rows, &pb.DriftRow{
			Pkg:      r.Package,
			Versions: versions,
			Level:    r.Level,
		})
	}
	return out, nil
}

func (s *scanServer) Dupes(_ context.Context, req *pb.DupesRequest) (*pb.DupesResponse, error) {
	groups, err := scan.Dupes(s.rootFor(req.GetRoot()))
	if err != nil {
		return nil, internalErr(err)
	}
	out := &pb.DupesResponse{Groups: make([]*pb.DupeGroup, 0, len(groups))}
	for _, g := range groups {
		sites := make([]*pb.DeclSite, 0, len(g.Sites))
		for _, d := range g.Sites {
			sites = append(sites, &pb.DeclSite{Repo: d.Repo, File: d.File, Line: d.Line})
		}
		out.Groups = append(out.Groups, &pb.DupeGroup{
			Name:      g.Name,
			Sites:     sites,
			RepoCount: g.RepoCount,
		})
	}
	return out, nil
}

func (s *scanServer) Clones(_ context.Context, req *pb.ClonesRequest) (*pb.ClonesResponse, error) {
	res, err := scan.Clones(s.rootFor(req.GetRoot()), req.GetSymbol(), req.GetMinTokens(), req.GetSameFile())
	if err != nil {
		return nil, internalErr(err)
	}
	out := &pb.ClonesResponse{
		Clusters:        make([]*pb.CloneCluster, 0, len(res.Clusters)),
		NpmOnlyPackages: res.NpmOnlyPackages,
	}
	for _, c := range res.Clusters {
		members := make([]*pb.CloneMember, 0, len(c.Members))
		for _, m := range c.Members {
			members = append(members, &pb.CloneMember{
				Repo: m.Repo, File: m.File, Line: m.Line, Name: m.Name,
				Exported: m.Exported, Kind: m.Kind, Sig: m.Sig,
			})
		}
		out.Clusters = append(out.Clusters, &pb.CloneCluster{
			Members:    members,
			TokenCount: c.TokenCount,
			Sig:        c.Sig,
			Literals:   c.Literals,
			Notes:      c.Notes,
		})
	}
	return out, nil
}

func (s *scanServer) Context(_ context.Context, req *pb.ContextRequest) (*pb.ContextResponse, error) {
	if req.GetSymbol() == "" {
		return nil, status.Error(codes.InvalidArgument, "symbol is required")
	}
	pack, err := scan.Context(s.rootFor(req.GetRoot()), req.GetSymbol(), req.GetFrom(), req.GetSites())
	if err != nil {
		return nil, internalErr(err)
	}
	out := &pb.ContextResponse{
		Symbol:     pack.Symbol,
		TotalSites: pack.TotalSites,
		TotalFiles: pack.TotalFiles,
		TotalRepos: pack.TotalRepos,
	}
	for _, p := range pack.PropCounts {
		out.PropCounts = append(out.PropCounts, &pb.PropCount{Name: p.Name, Count: p.Count})
	}
	if pack.Definition != nil {
		out.Definition = &pb.Definition{
			Repo: pack.Definition.Repo, File: pack.Definition.File,
			Line: pack.Definition.Line, Excerpt: pack.Definition.Excerpt,
		}
	}
	for _, e := range pack.Excerpts {
		out.Excerpts = append(out.Excerpts, &pb.UsageExcerpt{
			Repo: e.Repo, File: e.File, Line: e.Line, JsxProps: e.JsxProps, Code: e.Code,
		})
	}
	for _, g := range pack.SameName {
		out.SameName = append(out.SameName, &pb.SameNameGroup{
			Repo: g.Repo, Sites: g.Sites, FromHint: g.FromHint,
		})
	}
	return out, nil
}
