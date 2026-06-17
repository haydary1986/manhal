package bot

import (
	"sync"

	"github.com/erticaz/manhal/internal/cite"
	"github.com/erticaz/manhal/internal/scholar"
)

// sessionState is the per-user wizard state for multi-step flows (e.g. waiting
// for the user to send a DOI). It lives in the Telegram adapter because it is a
// conversational-UI concern, not core domain state.
type sessionState string

const (
	stateNone             sessionState = ""
	stateAwaitDOI         sessionState = "await_doi"
	stateAwaitQuery       sessionState = "await_query"
	stateAwaitJournal     sessionState = "await_journal"
	stateAwaitAuthor      sessionState = "await_author"
	stateAwaitOADOI       sessionState = "await_oa_doi"
	stateAwaitAdminLabel  sessionState = "await_admin_label"
	stateAwaitAIInput     sessionState = "await_ai_input"
	stateAwaitPromotion   sessionState = "await_promotion"
	stateAwaitPromoCount  sessionState = "await_promo_count"
	stateAwaitPublish     sessionState = "await_publish"
	stateAwaitLitReview   sessionState = "await_litreview"
	stateAwaitStats       sessionState = "await_stats"
	stateAwaitLatexFile   sessionState = "await_latex_file"
	stateAwaitSupport     sessionState = "await_support"
	stateAwaitGiftCode    sessionState = "await_gift_code"
	stateAwaitFollowTopic sessionState = "await_follow_topic"
	stateAwaitTrend       sessionState = "await_trend"
	stateAwaitGap         sessionState = "await_gap"
	stateAwaitVivaFile    sessionState = "await_viva_file"
	stateAwaitRadar       sessionState = "await_radar"
	stateAwaitSimilarity  sessionState = "await_similarity"
	stateAwaitSemantic    sessionState = "await_semantic"
	stateAwaitPdfFile     sessionState = "await_pdf_file"
	statePdfChat          sessionState = "pdf_chat"
)

// session holds one user's conversational context: the wizard state plus the
// most recent search/author results, referenced by index from inline buttons.
type session struct {
	state        sessionState
	results      []scholar.SearchResult
	authors      []scholar.Author
	adminParent  string             // parent id for an in-progress "add button" wizard
	adminLabel   string             // label captured during the "add button" wizard
	aiTool       string             // active AI tool key while awaiting input
	promoteRank  string             // chosen rank key while awaiting promotion activities
	promoDraft   map[string]float64 // accumulated counts in the interactive builder
	promoPending string             // activity key awaiting a count in the builder
	statsTest    string             // chosen statistical test while awaiting data
	lastWork     *cite.Work         // most recent fetched citation, for "save to library"
	lastAuthor   *scholar.Author    // most recent viewed author profile, for citation-watch
	pdfChunks    []pdfChunk         // embedded chunks of the active PDF (RAG, #24)
}

// sessions is a concurrency-safe map of Telegram user ID -> session.
type sessions struct {
	mu sync.Mutex
	m  map[int64]*session
}

func newSessions() *sessions {
	return &sessions{m: make(map[int64]*session)}
}

// entry returns the user's session, creating it on first use. Caller holds mu.
func (s *sessions) entry(userID int64) *session {
	e := s.m[userID]
	if e == nil {
		e = &session{}
		s.m[userID] = e
	}
	return e
}

func (s *sessions) get(userID int64) sessionState {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.m[userID]; e != nil {
		return e.state
	}
	return stateNone
}

func (s *sessions) set(userID int64, state sessionState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entry(userID).state = state
}

func (s *sessions) clear(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, userID)
}

// setResults stores search results and resets the state to none (the user is no
// longer mid-wizard; they now act on the results via buttons).
func (s *sessions) setResults(userID int64, results []scholar.SearchResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(userID)
	e.state = stateNone
	e.results = results
}

// resultAt returns the search result at index i, or false if out of range or
// the session has no results.
func (s *sessions) resultAt(userID int64, i int) (scholar.SearchResult, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.m[userID]
	if e == nil || i < 0 || i >= len(e.results) {
		return scholar.SearchResult{}, false
	}
	return e.results[i], true
}

// setAuthors stores author results and resets the state to none.
func (s *sessions) setAuthors(userID int64, authors []scholar.Author) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(userID)
	e.state = stateNone
	e.authors = authors
}

// authorAt returns the author at index i, or false if out of range.
func (s *sessions) authorAt(userID int64, i int) (scholar.Author, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.m[userID]
	if e == nil || i < 0 || i >= len(e.authors) {
		return scholar.Author{}, false
	}
	return e.authors[i], true
}

// startAdminLabel records the chosen parent and moves to label entry.
func (s *sessions) startAdminLabel(userID int64, parentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(userID)
	e.state = stateAwaitAdminLabel
	e.adminParent = parentID
}

// captureAdminLabel stores the label and clears the wizard state.
func (s *sessions) captureAdminLabel(userID int64, label string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(userID)
	e.state = stateNone
	e.adminLabel = label
}

// adminDraft returns the in-progress add-button parent and label.
func (s *sessions) adminDraft(userID int64) (parent, label string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.m[userID]; e != nil {
		return e.adminParent, e.adminLabel
	}
	return "", ""
}

// startAITool records the chosen AI tool and awaits the user's input text.
func (s *sessions) startAITool(userID int64, toolKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(userID)
	e.state = stateAwaitAIInput
	e.aiTool = toolKey
}

// aiTool returns the active AI tool key for the user.
func (s *sessions) aiTool(userID int64) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.m[userID]; e != nil {
		return e.aiTool
	}
	return ""
}

// startPromotion records the chosen rank and awaits the activities message.
func (s *sessions) startPromotion(userID int64, rankKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(userID)
	e.state = stateAwaitPromotion
	e.promoteRank = rankKey
}

// promoteRank returns the rank key chosen for the promotion calculation.
func (s *sessions) promoteRank(userID int64) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.m[userID]; e != nil {
		return e.promoteRank
	}
	return ""
}

// promoBegin starts the interactive builder for a rank with a fresh draft.
func (s *sessions) promoBegin(userID int64, rankKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(userID)
	e.state = stateNone
	e.promoteRank = rankKey
	e.promoDraft = map[string]float64{}
	e.promoPending = ""
}

// promoAwaitCount marks an activity key as awaiting its count from the user.
func (s *sessions) promoAwaitCount(userID int64, key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(userID)
	e.state = stateAwaitPromoCount
	e.promoPending = key
}

// promoPendingKey returns the key awaiting a count ("" if none).
func (s *sessions) promoPendingKey(userID int64) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.m[userID]; e != nil {
		return e.promoPending
	}
	return ""
}

// promoSetCount records (or clears, when n<=0) the count for a key and exits the
// awaiting-count state.
func (s *sessions) promoSetCount(userID int64, key string, n float64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(userID)
	if e.promoDraft == nil {
		e.promoDraft = map[string]float64{}
	}
	if n > 0 {
		e.promoDraft[key] = n
	} else {
		delete(e.promoDraft, key)
	}
	e.promoPending = ""
	e.state = stateNone
}

// promoDraftCounts returns a copy of the current draft counts.
func (s *sessions) promoDraftCounts(userID int64) map[string]float64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := map[string]float64{}
	if e := s.m[userID]; e != nil {
		for k, v := range e.promoDraft {
			out[k] = v
		}
	}
	return out
}

// promoResetDraft clears the draft counts but keeps the chosen rank.
func (s *sessions) promoResetDraft(userID int64) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.m[userID]; e != nil {
		e.promoDraft = map[string]float64{}
		e.promoPending = ""
	}
}

// startStats records the chosen statistical test and awaits the data message.
func (s *sessions) startStats(userID int64, testKey string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(userID)
	e.state = stateAwaitStats
	e.statsTest = testKey
}

// statsTest returns the active statistical test key for the user.
func (s *sessions) statsTest(userID int64) string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.m[userID]; e != nil {
		return e.statsTest
	}
	return ""
}

// setLastWork records the most recently fetched citation (for saving it later).
func (s *sessions) setLastWork(userID int64, w *cite.Work) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entry(userID).lastWork = w
}

// lastWork returns the most recently fetched citation, or nil.
func (s *sessions) lastWork(userID int64) *cite.Work {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.m[userID]; e != nil {
		return e.lastWork
	}
	return nil
}

// setLastAuthor records the most recently viewed author profile.
func (s *sessions) setLastAuthor(userID int64, a *scholar.Author) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.entry(userID).lastAuthor = a
}

// lastAuthor returns the most recently viewed author profile, or nil.
func (s *sessions) lastAuthor(userID int64) *scholar.Author {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.m[userID]; e != nil {
		return e.lastAuthor
	}
	return nil
}

// startPdfChat stores the embedded PDF chunks and enters chat mode.
func (s *sessions) startPdfChat(userID int64, chunks []pdfChunk) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e := s.entry(userID)
	e.state = statePdfChat
	e.pdfChunks = chunks
}

// pdfChunks returns the active PDF's embedded chunks.
func (s *sessions) pdfChunks(userID int64) []pdfChunk {
	s.mu.Lock()
	defer s.mu.Unlock()
	if e := s.m[userID]; e != nil {
		return e.pdfChunks
	}
	return nil
}
