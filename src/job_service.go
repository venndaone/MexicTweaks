package main

// Services: controlled execution of changes.
//
// Order for every run that changes the system:
//   1. Windows restore point  (on failure: user must explicitly confirm to continue)
//   2. MEXXIC backup of all original values, written to disk and verified
//   3. Restores (if selected), then tweaks
//   4. Every tweak is verified against the real system state afterwards.
//      A tweak is only reported as successful if the value is really set.

import (
	"errors"
	"fmt"
	"sync"
	"time"
)

type Job struct {
	Running    bool      `json:"running"`
	Finished   bool      `json:"finished"`
	Kind       string    `json:"kind"` // apply | restore | rp
	Done       int       `json:"done"`
	Total      int       `json:"total"`
	Current    string    `json:"current"` // code for the UI
	CurrentArg string    `json:"currentArg"`
	Log        []LogLine `json:"log"`
	NeedReboot bool      `json:"needReboot"`
	Errors     int       `json:"errors"`
	Await      bool      `json:"await"`
	AwaitMsg   string    `json:"awaitMsg"`
	Cancelled  bool      `json:"cancelled"`
}

var (
	job       Job
	jobMu     sync.Mutex
	confirmCh = make(chan bool, 1)
)

func jobUpdate(f func(j *Job)) { jobMu.Lock(); f(&job); jobMu.Unlock() }

func jobSnapshot() Job {
	jobMu.Lock()
	defer jobMu.Unlock()
	j := job
	j.Log = append([]LogLine{}, job.Log...)
	return j
}

func jobLog(kind, code, text string, args ...string) {
	l := LogLine{Kind: kind, Code: code, Args: args, Text: text, Time: time.Now()}
	jobUpdate(func(j *Job) {
		j.Log = append(j.Log, l)
		if kind == "err" {
			j.Errors++
		}
	})
	addActivity(l)
}

func step(code, arg string) { jobUpdate(func(j *Job) { j.Current, j.CurrentArg = code, arg }) }
func tick()                 { jobUpdate(func(j *Job) { j.Done++ }) }

func orderedTweaks(ids []string, reverse bool) []*Tweak {
	want := map[string]bool{}
	for _, id := range ids {
		want[id] = true
	}
	var out []*Tweak
	for _, t := range tweaks {
		if want[t.ID] {
			out = append(out, t)
		}
	}
	if reverse {
		for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
			out[i], out[j] = out[j], out[i]
		}
	}
	return out
}

func startJob(kind string, applyIDs, restoreIDs []string) error {
	jobMu.Lock()
	if job.Running {
		jobMu.Unlock()
		return errors.New("busy")
	}
	job = Job{Running: true, Kind: kind, Log: []LogLine{}}
	jobMu.Unlock()
	select { // drain stale confirmations
	case <-confirmCh:
	default:
	}
	go runChanges(kind, orderedTweaks(applyIDs, false), orderedTweaks(restoreIDs, true))
	return nil
}

func finishJob() {
	jobUpdate(func(j *Job) { j.Running, j.Finished, j.Current, j.Await = false, true, "", false })
	saveActivity()
}

func runChanges(kind string, apply, restore []*Tweak) {
	defer finishJob()
	admin := isAdmin()
	total := 1 + len(apply) + len(restore)
	if len(apply) > 0 {
		total++
	}
	jobUpdate(func(j *Job) { j.Total = total })

	// ---- 1. Windows restore point ----
	step("rp_creating", "")
	rpOK := true
	rpMsg := ""
	if err := createRestorePoint(); err != nil {
		rpOK, rpMsg = false, err.Error()
		jobLog("err", "rp_fail", "Restore point failed: "+rpMsg, rpMsg)
		if kind == "rp" {
			tick()
			return
		}
		// never continue blindly: ask the user
		jobUpdate(func(j *Job) { j.Await, j.AwaitMsg = true, rpMsg })
		ok := <-confirmCh
		jobUpdate(func(j *Job) { j.Await = false })
		if !ok {
			jobUpdate(func(j *Job) { j.Cancelled = true })
			jobLog("warn", "cancelled", "Cancelled, no changes were made")
			return
		}
		jobLog("warn", "rp_continue", "Continuing with MEXXIC backup only")
	} else {
		jobLog("ok", "rp_ok", "Restore point created")
	}
	tick()
	refreshRestorePoints()
	if kind == "rp" {
		return
	}

	// tweaks that need admin rights are skipped up-front when not elevated (nothing is touched)
	if !admin {
		var keep []*Tweak
		for _, t := range apply {
			if t.RequiresAdmin {
				jobLog("err", "admin_required", "Administrator rights required: "+t.Name, t.ID)
				tick()
				continue
			}
			keep = append(keep, t)
		}
		apply = keep
	}

	// ---- 2. MEXXIC backup ----
	var sess *Session
	if len(apply) > 0 {
		step("backup", "")
		storeMu.Lock()
		sess = &Session{ID: newSessionID(), Date: time.Now(), RestorePoint: rpOK, RestorePointMsg: rpMsg}
		var added []string
		for _, t := range apply {
			if _, ok := store.Tweaks[t.ID]; ok {
				continue // original values from an earlier run are kept
			}
			b := t.Snapshot()
			b.CreatedAt = time.Now()
			b.SessionID = sess.ID
			store.Tweaks[t.ID] = b
			added = append(added, t.ID)
		}
		store.Sessions = append(store.Sessions, sess)
		err := saveStoreLocked()
		if err != nil {
			for _, id := range added {
				delete(store.Tweaks, id)
			}
			store.Sessions = store.Sessions[:len(store.Sessions)-1]
			storeMu.Unlock()
			jobLog("err", "backup_fail", "Backup failed, no changes were made: "+err.Error(), err.Error())
			return
		}
		storeMu.Unlock()
		jobLog("ok", "backup_ok", fmt.Sprintf("Backup completed (%d tweaks)", len(apply)), fmt.Sprint(len(apply)))
		tick()
	}

	// ---- 3a. Restores ----
	restoredAny := false
	for _, t := range restore {
		step("restoring", t.ID)
		if t.RequiresAdmin && !admin {
			jobLog("err", "admin_required", "Administrator rights required: "+t.Name, t.ID)
			tick()
			continue
		}
		if err := restoreTweak(t); err != nil {
			jobLog("err", "restore_fail", "Failed to restore "+t.Name+": "+err.Error(), t.ID, err.Error())
		} else {
			restoredAny = true
			jobLog("ok", "restore_ok", "Restored: "+t.Name, t.ID)
			if t.RequiresRestart {
				markRestartNeeded()
			}
		}
		tick()
		time.Sleep(80 * time.Millisecond)
	}

	// ---- 3b. Tweaks ----
	catTotal, catFail := map[string]int{}, map[string]int{}
	for _, t := range apply {
		step("applying", t.ID)
		catTotal[t.Category]++
		st := &SessionTweak{ID: t.ID, Name: t.Name, Category: t.Category}
		err := applyTweak(t, st)
		storeMu.Lock()
		sess.Tweaks = append(sess.Tweaks, st)
		saveStoreLocked()
		storeMu.Unlock()
		if err != nil {
			catFail[t.Category]++
			jobLog("err", "tweak_fail", "Failed to apply "+t.Name+": "+err.Error(), t.ID, err.Error())
		} else {
			jobLog("ok", "tweak_ok", "Applied: "+t.Name, t.ID)
			if t.RequiresRestart {
				markRestartNeeded()
			}
		}
		tick()
		time.Sleep(80 * time.Millisecond)
	}

	// ---- 4. Summary per category ----
	for _, c := range categoryOrder {
		if catTotal[c] == 0 {
			continue
		}
		if catFail[c] == 0 {
			jobLog("ok", "cat_ok", c+" tweaks applied", c)
		} else {
			jobLog("err", "cat_fail", fmt.Sprintf("%s tweaks: %d of %d failed", c, catFail[c], catTotal[c]), c, fmt.Sprint(catFail[c]), fmt.Sprint(catTotal[c]))
		}
	}
	if restoredAny {
		jobLog("ok", "restore_done", "Changes restored")
	}
	if restartNeeded() {
		jobUpdate(func(j *Job) { j.NeedReboot = true })
	}
}

// applyTweak: backup must exist and be on disk BEFORE anything is changed.
func applyTweak(t *Tweak, st *SessionTweak) error {
	storeMu.Lock()
	b := store.Tweaks[t.ID]
	if b == nil {
		storeMu.Unlock()
		st.Error = "no backup"
		return errors.New("no backup, tweak skipped")
	}
	if t.prepare != nil {
		t.prepare(b)
		if err := saveStoreLocked(); err != nil {
			storeMu.Unlock()
			st.Error = "backup failed"
			return errors.New("backup failed, tweak skipped: " + err.Error())
		}
	}
	storeMu.Unlock()

	before := t.State()
	err := t.Apply(b)
	after := t.State()
	changes := diffChanges(before, after)

	storeMu.Lock()
	b.AppliedAt = time.Now()
	if len(b.Changes) == 0 {
		b.Changes = changes
	}
	storeMu.Unlock()

	st.Changes = changes
	if err == nil && !t.Active() {
		err = errors.New("verification failed: value was not set")
	}
	if err != nil {
		st.Error = err.Error()
		return err
	}
	st.OK = true
	return nil
}

func restoreTweak(t *Tweak) error {
	storeMu.Lock()
	b := store.Tweaks[t.ID]
	storeMu.Unlock()
	if b == nil {
		return errors.New("no backup available")
	}
	if err := t.Restore(b); err != nil {
		return err
	}
	storeMu.Lock()
	delete(store.Tweaks, t.ID)
	markRestored(t.ID)
	err := saveStoreLocked()
	storeMu.Unlock()
	return err
}
