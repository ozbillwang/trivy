package arm

import (
	"context"
	"fmt"
	"io/fs"
	"sync"

	"github.com/aquasecurity/trivy/pkg/iac/adapters/arm"
	"github.com/aquasecurity/trivy/pkg/iac/framework"
	"github.com/aquasecurity/trivy/pkg/iac/rego"
	"github.com/aquasecurity/trivy/pkg/iac/rules"
	"github.com/aquasecurity/trivy/pkg/iac/scan"
	"github.com/aquasecurity/trivy/pkg/iac/scanners"
	"github.com/aquasecurity/trivy/pkg/iac/scanners/azure"
	"github.com/aquasecurity/trivy/pkg/iac/scanners/azure/arm/parser"
	"github.com/aquasecurity/trivy/pkg/iac/scanners/options"
	"github.com/aquasecurity/trivy/pkg/iac/state"
	"github.com/aquasecurity/trivy/pkg/iac/types"
	"github.com/aquasecurity/trivy/pkg/log"
)

var _ scanners.FSScanner = (*Scanner)(nil)
var _ options.ConfigurableScanner = (*Scanner)(nil)

type Scanner struct {
	mu                      sync.Mutex
	scannerOptions          []options.ScannerOption
	logger                  *log.Logger
	frameworks              []framework.Framework
	regoOnly                bool
	regoScanner             *rego.Scanner
	includeDeprecatedChecks bool
}

func (s *Scanner) SetIncludeDeprecatedChecks(b bool) {
	s.includeDeprecatedChecks = b
}

func (s *Scanner) SetRegoOnly(regoOnly bool) {
	s.regoOnly = regoOnly
}

func New(opts ...options.ScannerOption) *Scanner {
	scanner := &Scanner{
		scannerOptions: opts,
		logger:         log.WithPrefix("azure-arm"),
	}
	for _, opt := range opts {
		opt(scanner)
	}
	return scanner
}

func (s *Scanner) Name() string {
	return "Azure ARM"
}

func (s *Scanner) SetFrameworks(frameworks []framework.Framework) {
	s.frameworks = frameworks
}

func (s *Scanner) initRegoScanner(srcFS fs.FS) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.regoScanner != nil {
		return nil
	}
	regoScanner := rego.NewScanner(types.SourceCloud, s.scannerOptions...)
	if err := regoScanner.LoadPolicies(srcFS); err != nil {
		return err
	}
	s.regoScanner = regoScanner
	return nil
}

func (s *Scanner) ScanFS(ctx context.Context, fsys fs.FS, dir string) (scan.Results, error) {
	p := parser.New(fsys)
	deployments, err := p.ParseFS(ctx, dir)
	if err != nil {
		return nil, err
	}
	if err := s.initRegoScanner(fsys); err != nil {
		return nil, err
	}

	return s.scanDeployments(ctx, deployments, fsys)
}

func (s *Scanner) scanDeployments(ctx context.Context, deployments []azure.Deployment, f fs.FS) (scan.Results, error) {

	var results scan.Results

	for _, deployment := range deployments {

		result, err := s.scanDeployment(ctx, deployment, f)
		if err != nil {
			return nil, err
		}
		results = append(results, result...)
	}

	return results, nil
}

func (s *Scanner) scanDeployment(ctx context.Context, deployment azure.Deployment, fsys fs.FS) (scan.Results, error) {
	var results scan.Results
	deploymentState := s.adaptDeployment(ctx, deployment)
	if !s.regoOnly {
		for _, rule := range rules.GetRegistered(s.frameworks...) {
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			default:
			}

			if !s.includeDeprecatedChecks && rule.Deprecated {
				continue // skip deprecated checks
			}

			ruleResults := rule.Evaluate(deploymentState)
			if len(ruleResults) > 0 {
				results = append(results, ruleResults...)
			}
		}
	}

	regoResults, err := s.regoScanner.ScanInput(ctx, rego.Input{
		Path:     deployment.Metadata.Range().GetFilename(),
		FS:       fsys,
		Contents: deploymentState.ToRego(),
	})
	if err != nil {
		return nil, fmt.Errorf("rego scan error: %w", err)
	}

	return append(results, regoResults...), nil
}

func (s *Scanner) adaptDeployment(ctx context.Context, deployment azure.Deployment) *state.State {
	return arm.Adapt(ctx, deployment)
}
