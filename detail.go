package main

import (
	"fmt"
	"strings"
	"time"

	"github.com/charmbracelet/lipgloss"
)

type detailModel struct {
	run         *RunInfo // nil if nothing selected
	logTail     []string
	logTailErr  error
	logTailStep string // for the header
	width       int
}

func newDetail() *detailModel { return &detailModel{} }

func (d *detailModel) SetRun(r *RunInfo) {
	d.run = r
	d.logTail = nil
	d.logTailErr = nil
	d.logTailStep = ""
}

func (d *detailModel) SetLogTail(step string, lines []string, err error) {
	d.logTailStep = step
	d.logTail = lines
	d.logTailErr = err
}

func (d *detailModel) SetSize(width int) { d.width = width }

func (d *detailModel) View() string {
	if d.run == nil {
		return dimStyle.Render("  select a run")
	}
	r := *d.run

	var b strings.Builder
	// Header
	b.WriteString(repoStyle.Render(fmt.Sprintf("%s/%s", r.Repo, r.Workflow)))
	b.WriteString("\n")

	// Metadata
	b.WriteString(kv("branch", branchStyle.Render(r.Run.HeadBranch)))
	if r.Run.Status == "completed" {
		b.WriteString(kv("duration", formatDuration(r.Run.UpdatedAt.Sub(r.Run.RunStartedAt))))
		b.WriteString(kv("finished", formatTimeAgo(time.Since(r.Run.UpdatedAt))+" ago"))
	} else {
		b.WriteString(kv("elapsed", formatDuration(time.Since(r.Run.RunStartedAt))))
	}

	// Jobs
	b.WriteString("\n")
	b.WriteString(sectionLabelStyle.Render("JOBS"))
	b.WriteString("\n")
	if len(r.Jobs) == 0 {
		b.WriteString(dimStyle.Render("  —"))
		b.WriteString("\n")
	} else {
		for _, j := range r.Jobs {
			b.WriteString("  ")
			b.WriteString(renderJobLine(j, 0))
			b.WriteString("\n")
		}
	}

	// Log tail
	if d.shouldShowLogTail() {
		b.WriteString("\n")
		head := "LOG TAIL"
		if d.logTailStep != "" {
			head = "LOG TAIL · " + d.logTailStep
		}
		b.WriteString(sectionLabelStyle.Render(head))
		b.WriteString("\n")
		if d.logTailErr != nil {
			b.WriteString(dimStyle.Render("  —"))
			b.WriteString("\n")
		} else if len(d.logTail) == 0 {
			b.WriteString(dimStyle.Render("  loading…"))
			b.WriteString("\n")
		} else {
			for _, line := range d.logTail {
				b.WriteString("  ")
				b.WriteString(colorizeLogLine(line))
				b.WriteString("\n")
			}
		}
	}

	return b.String()
}

// shouldShowLogTail hides the block for successful completed runs.
func (d *detailModel) shouldShowLogTail() bool {
	if d.run == nil {
		return false
	}
	r := d.run.Run
	if r.Status == "completed" && r.Conclusion == "success" {
		return false
	}
	return true
}

func kv(k, v string) string {
	key := lipgloss.NewStyle().Foreground(lipgloss.Color("245")).Width(10).Render(k)
	return "  " + key + v + "\n"
}
