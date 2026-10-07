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
	// 只规范化文本（去首尾空白）；整数量测保持原值，越界由 NewInspection 整体拒绝。
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
	// 关注项只从这份已校验、将被保存的记录派生，保证回执、列表与概览同源一致。
	record.Concerns = DeriveConcerns(record)
	return s.store.AddInspection(hiveID, record)
}

func (s *Service) List(hiveID string) ([]domain.Inspection, error) {
	return s.store.ListInspections(hiveID)
}
