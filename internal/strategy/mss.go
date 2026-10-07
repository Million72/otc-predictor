package strategy

import "otc-predictor/pkg/types"

// DetectMSS is the ICT/SMC term for what CHoCH (in structure.go) already
// detects — same underlying math, aliased here so entry-model code reads
// naturally as "MSS" per the requested terminology, with the equivalence
// documented rather than silently duplicated.
func DetectMSS(candles []types.Candle, structure StructureResult) *SweepResult {
	return CHoCH(candles, structure)
}
