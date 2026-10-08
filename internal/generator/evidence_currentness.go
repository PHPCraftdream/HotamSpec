package generator

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/PHPCraftdream/HotamSpec/internal/conformance"
	"github.com/PHPCraftdream/HotamSpec/internal/evidence"
	"github.com/PHPCraftdream/HotamSpec/internal/gate"
	"github.com/PHPCraftdream/HotamSpec/internal/ontology"
)

const evidenceCaseMarker = "<!-- hotam-evidence-case: "

// EvidenceCaseIdentity is invocation-local shared provenance. Compute it once
// for a bundle, never once per case or in a process-global cache.
type EvidenceCaseIdentity struct {
	source       string
	contexts     map[string][]evidence.Context
	engine       string
	declarations string
}

type evidenceCaseIntegrity struct {
	Version            int    `json:"version"`
	Language           string `json:"language"`
	Target             string `json:"target"`
	AtomID             string `json:"atom_id"`
	CaseID             string `json:"case_id"`
	SourceSHA256       string `json:"source_sha256"`
	EngineSHA256       string `json:"engine_sha256"`
	DeclarationsSHA256 string `json:"declarations_sha256"`
	ProofSHA256        string `json:"proof_sha256"`
	BodySHA256         string `json:"body_sha256"`
}

func identitySHA(value any) (string, error) {
	data, err := json.Marshal(value)
	if err != nil {
		return "", err
	}
	return bodySHA(string(data)), nil
}

func bodySHA(body string) string {
	sum := sha256.Sum256([]byte(body))
	return hex.EncodeToString(sum[:])
}

// NewEvidenceCaseIdentity binds actual authored inputs and engine provenance.
// Declaration-only graphs are useful pure-render inputs; their explicit identity
// still binds every declaration, and never pretends to identify missing source.
func NewEvidenceCaseIdentity(g *ontology.Graph, report evidence.Report) (*EvidenceCaseIdentity, error) {
	if g == nil {
		return nil, errors.New("case evidence identity requires a graph")
	}
	identity := &EvidenceCaseIdentity{}
	var err error
	if g.DomainDir != "" {
		identity.source, err = gate.ExecutionInputsFingerprint(g)
		if err != nil {
			return nil, fmt.Errorf("case evidence source identity: %w", err)
		}
	} else {
		identity.source, err = identitySHA("declaration-only graph")
		if err != nil {
			return nil, err
		}
	}
	// Use the existing engine-doc source fingerprint where available. A deployed
	// standalone binary has no source checkout: bind its actual executable bytes.
	_, file, _, available := runtime.Caller(0)
	if available {
		identity.engine, err = gate.EngineDocsFingerprint(filepath.Join(filepath.Dir(file), "..", ".."))
	}
	if !available || err != nil {
		path, executableErr := os.Executable()
		if executableErr != nil {
			return nil, executableErr
		}
		binary, openErr := os.Open(path)
		if openErr != nil {
			return nil, openErr
		}
		digest := sha256.New()
		_, readErr := io.Copy(digest, binary)
		closeErr := binary.Close()
		if err := errors.Join(readErr, closeErr); err != nil {
			return nil, err
		}
		identity.engine = hex.EncodeToString(digest.Sum(nil))
	}
	// Sources include exact integrity pins/status, not runtime diagnostic wording.
	type sourceIdentity struct{ ID, Anchor, Version, Expected, Actual, Status string }
	sources := make([]sourceIdentity, len(report.Sources))
	for index, item := range report.Sources {
		sources[index] = sourceIdentity{item.SourceID, item.Anchor, item.Version, item.ExpectedSHA256, item.ActualSHA256, item.Status}
	}
	identity.declarations, err = identitySHA(struct {
		Graph           *ontology.Graph
		Conformance     *ontology.ConformanceConfig
		Languages       []string
		DefaultLanguage string
		SchemaVersion   int
		Sources         []sourceIdentity
	}{g, g.Conformance, g.Languages, g.DefaultLanguage, report.SchemaVersion, sources})
	if err != nil {
		return nil, err
	}
	identity.contexts = make(map[string][]evidence.Context)
	for _, requirement := range report.Requirements {
		for _, test := range requirement.Tests {
			for _, artifact := range test.Artifacts {
				if artifact.CaseID == "" {
					continue
				}
				context := artifact.Context
				context.RecordedCaseFingerprint, context.CaseFingerprint = "", ""
				identity.contexts[artifact.CaseID] = append(identity.contexts[artifact.CaseID], context)
			}
		}
	}
	return identity, nil
}

func observedKind(value *ontology.ObservedValue) string {
	if value == nil {
		return "absent"
	}
	return value.Kind
}

func caseProofSHA(item conformance.CaseAssessment, contexts []evidence.Context) (string, error) {
	type comparisonProof struct {
		Name                                string
		Passed                              bool
		InputKind, ActualKind, ExpectedKind string
	}
	type executionProof struct {
		Metadata    conformance.Execution
		Comparisons []comparisonProof
	}
	executions := make([]executionProof, len(item.Executions))
	for index, execution := range item.Executions {
		metadata := execution
		metadata.Comparisons = nil
		executions[index].Metadata = metadata
		for _, comparison := range execution.Comparisons {
			executions[index].Comparisons = append(executions[index].Comparisons, comparisonProof{comparison.Name, comparison.Passed, observedKind(comparison.Input), observedKind(comparison.Actual), observedKind(comparison.Expected)})
		}
	}
	item.Executions = nil
	return identitySHA(struct {
		Case       conformance.CaseAssessment
		Executions []executionProof
		Contexts   []evidence.Context
	}{item, executions, contexts})
}

func (identity *EvidenceCaseIdentity) Stamp(report evidence.Report, item conformance.CaseAssessment, language, target, body string) (string, error) {
	if identity == nil || item.ID == "" || item.AtomID == "" || target == "" {
		return "", errors.New("case journal identity is incomplete")
	}
	proof, err := caseProofSHA(item, identity.contexts[item.ID])
	if err != nil {
		return "", err
	}
	marker := evidenceCaseIntegrity{1, language, target, item.AtomID, item.ID, identity.source, identity.engine, identity.declarations, proof, bodySHA(body)}
	data, err := json.Marshal(marker)
	if err != nil {
		return "", err
	}
	return evidenceCaseMarker + string(data) + " -->\n" + body, nil
}

func readEvidenceCaseIntegrity(document string) (evidenceCaseIntegrity, error) {
	var marker evidenceCaseIntegrity
	line, body, exists := strings.Cut(document, "\n")
	if !exists || !strings.HasPrefix(line, evidenceCaseMarker) || !strings.HasSuffix(line, " -->") {
		return marker, errors.New("missing or malformed case journal integrity marker")
	}
	encoded := strings.TrimSuffix(strings.TrimPrefix(line, evidenceCaseMarker), " -->")
	decoder := json.NewDecoder(strings.NewReader(encoded))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&marker); err != nil {
		return marker, fmt.Errorf("invalid case journal integrity metadata: %w", err)
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		return marker, errors.New("extra case journal integrity metadata")
	}
	canonical, err := json.Marshal(marker)
	if err != nil || string(canonical) != encoded {
		return marker, errors.New("noncanonical or duplicate case journal integrity metadata")
	}
	if marker.Version != 1 || marker.Language == "" || marker.Target == "" || marker.AtomID == "" || marker.CaseID == "" {
		return marker, errors.New("incomplete case journal integrity metadata")
	}
	for _, digest := range []string{marker.SourceSHA256, marker.EngineSHA256, marker.DeclarationsSHA256, marker.ProofSHA256, marker.BodySHA256} {
		decoded, err := hex.DecodeString(digest)
		if err != nil || len(decoded) != sha256.Size {
			return marker, errors.New("invalid case journal identity checksum")
		}
	}
	if marker.BodySHA256 != bodySHA(body) {
		return marker, errors.New("published case journal body checksum mismatch")
	}
	return marker, nil
}

// EvidenceCasePageCurrent validates exact body checksums and stable provenance.
// Raw diagnostics may differ between executions; published bytes stay intact.
func EvidenceCasePageCurrent(published, current string) error {
	old, err := readEvidenceCaseIntegrity(published)
	if err != nil {
		return err
	}
	fresh, err := readEvidenceCaseIntegrity(current)
	if err != nil {
		return err
	}
	old.BodySHA256, fresh.BodySHA256 = "", ""
	if old != fresh {
		return errors.New("case journal logical source/profile/declaration/proof identity is stale")
	}
	return nil
}
