package domain

import "time"

type BroodPattern string

const (
	BroodSolid  BroodPattern = "solid"
	BroodPatchy BroodPattern = "patchy"
	BroodNone   BroodPattern = "none"
)

type Temper string

const (
	TemperCalm       Temper = "calm"
	TemperNervous    Temper = "nervous"
	TemperAggressive Temper = "aggressive"
)

// Inspection 是一次现场巡检的固化结果，关注项由量测派生。
type Inspection struct {
	ID             string       `json:"id"`
	HiveID         string       `json:"hiveId"`
	ObservedAt     time.Time    `json:"observedAt"`
	ColonyStrength int          `json:"colonyStrength"`
	BroodPattern   BroodPattern `json:"broodPattern"`
	Temper         Temper       `json:"temper"`
	MiteCount      int          `json:"miteCount"`
	Note           string       `json:"note"`
	Concerns       []string     `json:"concerns"`
}

const (
	MinColonyStrength = 1
	MaxColonyStrength = 5
	MinMiteCount      = 0
	MaxMiteCount      = 60
	MaxNoteRunes      = 200
)

// InspectionMeasurements 是一次巡检请求里提交的原始量测。
type InspectionMeasurements struct {
	ColonyStrength int
	BroodPattern   BroodPattern
	Temper         Temper
	MiteCount      int
	Note           string
}

// NormalizeInspectionMeasurements 整理提交量测的文本表示：枚举与备注去掉首尾空白。
// 整数量测保持提交原值，越界值由 NewInspection 的范围校验整体拒绝，
// 不能在此改写成看似合法的值再保存。
func NormalizeInspectionMeasurements(input InspectionMeasurements) InspectionMeasurements {
	normalized := input
	normalized.BroodPattern = CanonicalBroodPattern(input.BroodPattern)
	normalized.Temper = CanonicalTemper(input.Temper)
	normalized.Note = TrimText(input.Note)
	return normalized
}

// CanonicalBroodPattern 去掉子脾形态文本的首尾空白。
func CanonicalBroodPattern(value BroodPattern) BroodPattern {
	return BroodPattern(TrimText(string(value)))
}

// CanonicalTemper 去掉性情文本的首尾空白。
func CanonicalTemper(value Temper) Temper {
	return Temper(TrimText(string(value)))
}

func NewInspection(
	hiveID string,
	observedAt time.Time,
	strength int,
	brood BroodPattern,
	temper Temper,
	miteCount int,
	note string,
) (Inspection, error) {
	if err := ValidateIdentifier("hiveId", hiveID, 3, 40); err != nil {
		return Inspection{}, err
	}
	if observedAt.IsZero() {
		return Inspection{}, Invalid("observedAt", "observedAt 不能为空")
	}
	if err := ValidateIntRange("colonyStrength", strength, MinColonyStrength, MaxColonyStrength); err != nil {
		return Inspection{}, err
	}
	if !ValidBroodPattern(brood) {
		return Inspection{}, Invalid("broodPattern", "broodPattern 必须是 %s、%s 或 %s", BroodSolid, BroodPatchy, BroodNone)
	}
	if !ValidTemper(temper) {
		return Inspection{}, Invalid("temper", "temper 必须是 %s、%s 或 %s", TemperCalm, TemperNervous, TemperAggressive)
	}
	if err := ValidateIntRange("miteCount", miteCount, MinMiteCount, MaxMiteCount); err != nil {
		return Inspection{}, err
	}
	if err := ValidateText("note", note, 0, MaxNoteRunes); err != nil {
		return Inspection{}, err
	}
	return Inspection{
		HiveID:         TrimText(hiveID),
		ObservedAt:     observedAt.UTC(),
		ColonyStrength: strength,
		BroodPattern:   brood,
		Temper:         temper,
		MiteCount:      miteCount,
		Note:           TrimText(note),
		Concerns:       []string{},
	}, nil
}

func ValidBroodPattern(value BroodPattern) bool {
	switch value {
	case BroodSolid, BroodPatchy, BroodNone:
		return true
	default:
		return false
	}
}

func ValidTemper(value Temper) bool {
	switch value {
	case TemperCalm, TemperNervous, TemperAggressive:
		return true
	default:
		return false
	}
}
