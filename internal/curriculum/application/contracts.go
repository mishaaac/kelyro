package application

import (
	"context"
	"fmt"
	"io"

	"github.com/mishaaac/kelyro/internal/curriculum"
	"github.com/mishaaac/kelyro/internal/research"
)

type CurriculumRepository interface {
	Add(context.Context, curriculum.CurriculumDefinition) error
	Get(context.Context, curriculum.CurriculumID, curriculum.CurriculumVersion) (curriculum.CurriculumDefinition, error)
	List(context.Context) ([]curriculum.CurriculumDefinition, error)
}

type PackActivation struct {
	PackID      curriculum.ID
	Version     curriculum.PackVersion
	ActivatedAt curriculum.Timestamp
}

func (activation PackActivation) Validate() error {
	if err := activation.PackID.Validate(); err != nil {
		return fmt.Errorf("pack activation: %w", err)
	}
	if err := activation.Version.Validate(); err != nil {
		return err
	}
	if err := activation.ActivatedAt.Validate(); err != nil {
		return fmt.Errorf("pack activation time: %w", err)
	}
	return nil
}

type PackRepository interface {
	Add(context.Context, curriculum.LearningPack) error
	Get(context.Context, curriculum.ID, curriculum.PackVersion) (curriculum.LearningPack, error)
	List(context.Context) ([]curriculum.LearningPack, error)
	Activate(context.Context, PackActivation) error
	Active(context.Context) (curriculum.LearningPack, error)
}

// PackCatalogRepository stores discovery metadata only. It never installs or
// activates a pack, and its manifests do not make pack contents trusted.
type PackCatalogRepository interface {
	Replace(context.Context, []curriculum.PackManifest) error
	List(context.Context) ([]curriculum.PackManifest, error)
	FindByID(context.Context, curriculum.ID) ([]curriculum.PackManifest, error)
}

type EnvironmentPackRepository interface {
	Add(context.Context, curriculum.EnvironmentPack) error
	Get(context.Context, curriculum.ID, curriculum.PackVersion) (curriculum.EnvironmentPack, error)
	List(context.Context) ([]curriculum.EnvironmentPack, error)
}

type CompilationRecord struct {
	ID        curriculum.ID
	Input     curriculum.CompilationInput
	Config    curriculum.CompilationConfig
	Result    curriculum.CompilationResult
	CreatedAt curriculum.Timestamp
}

func (record CompilationRecord) Validate() error {
	if err := record.ID.Validate(); err != nil {
		return fmt.Errorf("compilation record: %w", err)
	}
	if err := record.Config.Validate(); err != nil {
		return err
	}
	if err := record.Input.Validate(record.Config.SourcePolicy); err != nil {
		return err
	}
	if err := record.Result.Validate(); err != nil {
		return err
	}
	if record.Input.Goal.ID != record.Result.Curriculum.Goal.ID {
		return fmt.Errorf("compilation input and result goals differ")
	}
	if err := record.CreatedAt.Validate(); err != nil {
		return fmt.Errorf("compilation record time: %w", err)
	}
	return nil
}

type CompilationRepository interface {
	Append(context.Context, CompilationRecord) error
	Get(context.Context, curriculum.ID) (CompilationRecord, error)
	ListByCurriculum(context.Context, curriculum.CurriculumID) ([]CompilationRecord, error)
}

// ResearchBundleProvider is a read-only adapter over durable I-03 records.
// Implementations must not perform discovery, fetching, or other network I/O.
type ResearchBundleProvider interface {
	GetBundle(context.Context, research.ID) (research.SourceBundle, error)
	GetClaim(context.Context, research.ClaimID) (research.Claim, error)
}

type Clock interface {
	Now() curriculum.Timestamp
}

type FileMetadata struct {
	Path      string
	Size      int64
	Directory bool
	Regular   bool
	Symlink   bool
}

type Filesystem interface {
	Open(context.Context, string) (io.ReadCloser, error)
	Metadata(context.Context, string) (FileMetadata, error)
	ReadDirectory(context.Context, string) ([]FileMetadata, error)
}

type PackSource struct {
	Path string
}

type PackArchiveEntry struct {
	Name      string
	Size      int64
	Directory bool
	Symlink   bool
}

type PackArchive interface {
	Entries() []PackArchiveEntry
	Open(context.Context, string) (io.ReadCloser, error)
	Close() error
}

type PackArchiveReader interface {
	Open(context.Context, PackSource) (PackArchive, error)
}

type SignatureEnvelope struct {
	PayloadHash string
	Signature   []byte
	KeyID       string
}

type SignatureVerification struct {
	Verified bool
	KeyID    string
	Reason   string
}

// SignatureVerifier is an optional future hook. Pack validation must remain
// safe when no signing ecosystem has been configured.
type SignatureVerifier interface {
	Verify(context.Context, SignatureEnvelope) (SignatureVerification, error)
}
