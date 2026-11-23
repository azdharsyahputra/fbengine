// Code generated from Metrica full tracking (audit/speed_table.js); DO NOT EDIT.
// Speed (m/s, over 0.5 s, live) per role group and phase: p80 caps a non-action move,
// p90 a marker away from goal. Role groups: 0 CB, 1 FB/WB, 2 DM/CM, 3 AM/W/M, 4 ST.
// Phases: 0 in possession settled, 1 in possession < 4 s after winning it, 2 defending settled,
// 3 defending < 4 s after losing it.

package engine

var teamShapeSpeedP80 = [5][4]float64{
	{2.91, 3.05, 3.41, 3.16}, // CB
	{3.16, 3.26, 3.61, 3.45}, // FB
	{3.54, 3.42, 3.85, 3.65}, // DM
	{3.60, 3.44, 3.68, 3.76}, // AM
	{3.40, 3.21, 3.22, 3.49}, // ST
}

var teamShapeSpeedP90 = [5][4]float64{
	{3.54, 3.81, 4.22, 3.94}, // CB
	{3.82, 3.93, 4.47, 4.37}, // FB
	{4.16, 4.08, 4.66, 4.40}, // DM
	{4.57, 4.28, 4.49, 4.65}, // AM
	{4.47, 4.12, 4.00, 4.33}, // ST
}
