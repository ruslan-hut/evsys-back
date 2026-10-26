package aireport

import (
	"evsys-back/entity"
	"os"
	"strings"
	"testing"
	"time"
)

func sampleReport(t *testing.T) *entity.AIReport {
	t.Helper()
	return &entity.AIReport{
		Id:         "r1",
		Model:      "gpt-6.1-sol",
		TimeZone:   "Europe/Madrid",
		PeriodFrom: mustUTC(t, "2026-10-18T22:00:00Z"),
		PeriodTo:   mustUTC(t, "2026-10-25T23:00:00Z"),
	}
}

func sampleSnapshot() *Snapshot {
	return &Snapshot{
		Scope: &SnapshotScope{Locations: []ScopeLocation{{Id: "PED", Name: "El Pedernoso"}}},
		Sessions: &SessionsSummary{
			Finished: 33, PrevFinished: 35,
			EnergyKWh: 1007.6, PrevEnergyKWh: 896.4,
			TariffPricePerKWhEUR: 0.45, TurnoverEstimateEUR: 397.46,
		},
		Stations: []*StationWeek{
			{ChargePointId: "PE00004", ConnectorErrors: 60, PrevConnectorErrs: 40},
			{ChargePointId: "PE00002", ConnectorErrors: 42, PrevConnectorErrs: 37},
		},
	}
}

func TestRenderCompact(t *testing.T) {
	report := sampleReport(t)
	analysis := &entity.AIAnalysis{Language: entity.MailLanguageES, Result: &entity.AIAnalysisResult{
		Status:   "critical",
		Headline: "Dos equipos en fallo",
		Diagnosis: []*entity.AIDiagnosis{
			{Subject: "PE00004", Text: "Los dos conectores están en fallo y no registraron sesiones durante la semana."},
			{Subject: "Pagos", Text: "Hay dos errores pendientes de conciliación."},
		},
		Actions: []*entity.AIAction{
			{Priority: entity.AIPriorityUrgent, Target: "PE00004", Action: "Diagnosticar controlador y ambos conectores."},
			{Priority: entity.AIPriorityHigh, Target: "Pagos", Action: "Conciliar las dos operaciones."},
		},
		ClosingNote: "no considerar recuperado un conector hasta verificar una carga real satisfactoria.",
	}}
	body := renderReport(report, analysis, sampleSnapshot())
	if out := os.Getenv("AIREPORT_SAMPLE"); out != "" {
		_ = os.WriteFile(out, []byte(body), 0o644)
	}
	for _, want := range []string{
		"WattBrews | Informe semanal",
		"El Pedernoso · 19–25 octubre 2026",
		"Intervención urgente",
		"1.007,6 kWh", "+12,4 %",
		"−5,7 % vs. semana anterior",
		"397,46 €", "A 0,45 €/kWh",
		">102<", "+32,5 % · antes: 77",
		"Diagnóstico ejecutivo", "Plan de actuación", "P1 · Urgente", "P2 · Prioritario",
		"Criterio de cierre:",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("email lacks %q", want)
		}
	}
}

func TestRenderLegacyFindings(t *testing.T) {
	analysis := &entity.AIAnalysis{Language: entity.MailLanguageEN, Result: &entity.AIAnalysisResult{
		Status:  "attention",
		Summary: "summary",
		Findings: []*entity.AIFinding{
			{Severity: "warning", ChargePointId: "PE00002", Title: "frequent disconnects", Recommendation: "check the SIM"},
			{Severity: "info", Title: "network note"},
		},
	}}
	body := renderReport(sampleReport(t), analysis, nil)
	for _, want := range []string{"frequent disconnects", "check the SIM", "P2 · High", "Whole network", "network note"} {
		if !strings.Contains(body, want) {
			t.Errorf("email lacks %q", want)
		}
	}
}

func TestPeriodLabel(t *testing.T) {
	es := textsFor(entity.MailLanguageES)
	tests := []struct{ from, to, want string }{
		{"2026-10-19", "2026-10-26", "19–25 octubre 2026"},
		{"2026-09-28", "2026-10-05", "28 septiembre – 4 octubre 2026"},
		{"2026-12-28", "2027-01-04", "28 diciembre 2026 – 3 enero 2027"},
		{"2026-10-25", "2026-10-26", "25 octubre 2026"},
	}
	for _, tt := range tests {
		from, _ := time.Parse(time.DateOnly, tt.from)
		to, _ := time.Parse(time.DateOnly, tt.to)
		got := periodLabel(&entity.AIReport{TimeZone: "UTC", PeriodFrom: from, PeriodTo: to}, es)
		if got != tt.want {
			t.Errorf("%s..%s = %q, want %q", tt.from, tt.to, got, tt.want)
		}
	}
}

func TestNumberFormat(t *testing.T) {
	es, en := textsFor(entity.MailLanguageES), textsFor(entity.MailLanguageEN)
	if got := number(1234567.891, 2, es); got != "1.234.567,89" {
		t.Errorf("es = %q", got)
	}
	if got := euro(397.46, en); got != "€397.46" {
		t.Errorf("en = %q", got)
	}
	if got := change(33, 35, es); got != "−5,7 %" {
		t.Errorf("change = %q", got)
	}
}
