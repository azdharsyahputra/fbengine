// Module switches for the FootballEngine (FE) modules ported on top of the
// Player12 skeleton (see /Users/csadeveloper/fm/referensi/FE_PORT_PLAN.md) and
// our M10 layer now live in EngineConfig.Modules (config.go), one copy per
// match. Each FE module keeps the original Player12 path behind its switch so
// the two can be compared on the same seed. The CLI maps FMENGINE_OFF /
// FMENGINE_ON onto them (ApplyOverrides).
// FootballEngine: https://github.com/DeltaBitsSystem/FootballEngine (GPL-3.0).
package engine
