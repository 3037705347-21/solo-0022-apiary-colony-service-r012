package inspection

import (
	"time"

	"example.com/apiary-colony-service/internal/domain"
	"example.com/apiary-colony-service/internal/store"
)

// Service 接收巡检、完成范围校验并派生关注项。
type Service struct {
	store *store.Memory
	clock func() time.Time
}

func New(memory *store.Memory, clock func() time.Time) *Service {
	if clock == nil {
		clock = time.Now
	}
	return &Service{store: memory, clock: clock}
}

type RecordInput struct {
	ObservedAt     time.Time
	ColonyStrength int
	BroodPattern   string
	Temper         string
	MiteCount      int
	Note           string
}

func (s *Service) Record(hiveID string, input RecordInput) (domain.Inspection, error) {
	observedAt := input.ObservedAt
	if observedAt.IsZero() {
		observedAt = s.clock()
	}
	submitted := domain.InspectionMeasurements{
		ColonyStrength: input.ColonyStrength,
		BroodPattern:   domain.BroodPattern(input.BroodPattern),
		Temper:         domain.Temper(input.Temper),
		MiteCount:      input.MiteCount,
		Note:           input.Note,
	}
	normalized := domain.NormalizeInspectionMeasurements(submitted)
	record, err := domain.NewInspection(
		hiveID,
		observedAt,
		normalized.ColonyStrength,
		normalized.BroodPattern,
		normalized.Temper,
		normalized.MiteCount,
		normalized.Note,
	)
	if err != nil {
		return domain.Inspection{}, err
	}
	record.Concerns = submittedConcerns(submitted)
	saved, err := s.store.AddInspection(hiveID, record)
	if err != nil {
		return domain.Inspection{}, err
	}
	saved.ColonyStrength = submitted.ColonyStrength
	saved.MiteCount = submitted.MiteCount
	return saved, nil
}

// submittedConcerns 按提交时的量测派生关注项。
func submittedConcerns(input domain.InspectionMeasurements) []string {
	return DeriveConcerns(measurementSnapshot(input))
}

// measurementSnapshot 把提交时的量测还原成一份用于派生关注项的记录。
func measurementSnapshot(input domain.InspectionMeasurements) domain.Inspection {
	return domain.Inspection{
		ColonyStrength: input.ColonyStrength,
		BroodPattern:   input.BroodPattern,
		Temper:         input.Temper,
		MiteCount:      input.MiteCount,
		Note:           input.Note,
	}
}

func (s *Service) List(hiveID string) ([]domain.Inspection, error) {
	return s.store.ListInspections(hiveID)
}
