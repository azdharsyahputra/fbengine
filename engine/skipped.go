// Ported from Player12 src/model/match_engine.cpp (highlight/interpolation/commentary hooks) and src/model/match_engine_tracking.cpp (GPL-3.0)

package engine

// Hooks the core engine calls into subsystems that are out of scope for this
// port (playback/highlights, interpolation, commentary text, tracking). They
// are output-only in the C++ (they never feed back into the simulation), so
// the no-op versions keep matches identical. Later stages must NOT
// re-implement these; the rest of the playback/highlight/debug-JSON API is
// simply not ported.

// recordHighlight: highlights are not ported (C++ 9648).
func (e *MatchEngine) recordHighlight(t MatchEventType) { _ = t }

// captureInterpolationFrame: render interpolation is not ported (C++ 1601).
func (e *MatchEngine) captureInterpolationFrame() {}

// describePendingEvents: commentary text is not ported (C++ 9636); events
// keep an empty Description.
func (e *MatchEngine) describePendingEvents() { e.undescribedEvents = 0 }

// Tracker feeds (match_engine_tracking.cpp): tracking logic is not ported.

func (e *MatchEngine) trackTouch(player *MatchPlayer) { _ = player }

func (e *MatchEngine) trackPassRelease(passer *MatchPlayer) { _ = passer }

func (e *MatchEngine) trackPassCompletion(passer *MatchPlayer, receiver *MatchPlayer) {}

func (e *MatchEngine) trackShot(shooter *MatchPlayer, xg float64, setPiece bool, header bool, penalty bool) {
}

func (e *MatchEngine) trackPressures() {}
