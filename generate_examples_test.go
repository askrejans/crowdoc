package main

import (
	"os"
	"testing"
)

// TestGenerateExampleXLSX creates the example XLSX file for the examples/ directory.
// Run with: go test -run TestGenerateExampleXLSX -v
func TestGenerateExampleXLSX(t *testing.T) {
	path := "examples/employee-data.xlsx"

	sheets := map[string][][]string{
		"Executive Summary": {
			{"The company completed a strong first half of 2026 with revenue growth of 35.9% year-over-year. All product lines showed positive momentum except Data Migration, which declined as expected due to the completion of major client onboarding projects."},
			{"Engineering headcount grew to 10 across three teams (Core Platform, DevOps, and Frontend). Key hires include two senior engineers and an architect, strengthening our capacity for the planned microservices migration in Q3."},
			{"Customer satisfaction reached an all-time high with NPS of 58, up from 45 at the start of the year. Churn rate decreased from 2.0% to 1.5%, driven by improved onboarding workflows and the launch of the priority support tier."},
			{"Looking ahead to H2, the primary focus areas are: (1) completing the platform migration to microservices, (2) launching the mobile companion app, and (3) expanding into the Nordic market with localized offerings."},
		},
		"Employees": {
			{"ID", "Name", "Department", "Position", "Start Date", "Salary (EUR)"},
			{"E001", "Anna Schmidt", "Engineering", "Senior Developer", "2022-03-15", "4200"},
			{"E002", "Erik Johansson", "Engineering", "Backend Lead", "2021-08-01", "4800"},
			{"E003", "Maria Torres", "Engineering", "Architect", "2020-11-10", "5200"},
			{"E004", "Sophie Martin", "Engineering", "Frontend Lead", "2022-01-20", "4600"},
			{"E005", "Robert Fischer", "DevOps", "SRE Engineer", "2023-02-01", "4000"},
			{"E006", "Klaus Weber", "DevOps", "Infrastructure", "2023-06-15", "3800"},
			{"E007", "Thomas Muller", "Management", "VP Engineering", "2019-04-01", "6500"},
			{"E008", "Sandra Nilsson", "Product", "Product Manager", "2022-09-01", "4400"},
			{"E009", "Jan Kowalski", "Sales", "Account Executive", "2023-01-15", "3200"},
			{"E010", "Diana Petrova", "Sales", "Sales Director", "2021-03-01", "5000"},
		},
		"Departments": {
			{"Department", "Head Count", "Budget Q1 (EUR)", "Budget Q2 (EUR)", "Location"},
			{"Engineering", "4", "82000", "88000", "Berlin HQ"},
			{"DevOps", "2", "38000", "42000", "Berlin HQ"},
			{"Management", "1", "22000", "22000", "Berlin HQ"},
			{"Product", "1", "18000", "19000", "Remote"},
			{"Sales", "2", "42000", "48000", "Munich Office"},
		},
		"Quarterly KPIs": {
			{"KPI", "Q1 Target", "Q1 Actual", "Q2 Target", "Q2 Actual"},
			{"Revenue (EUR)", "140000", "142600", "155000", "157000"},
			{"New Customers", "12", "15", "14", "18"},
			{"Churn Rate (%)", "2.0", "1.8", "2.0", "1.5"},
			{"NPS Score", "45", "52", "50", "58"},
			{"Uptime (%)", "99.9", "99.95", "99.9", "99.97"},
			{"Deploy Frequency", "4/week", "5/week", "5/week", "6/week"},
		},
	}

	charts := []xlsxTestChart{
		{
			Title:      "Revenue by Quarter",
			ChartType:  "bar",
			Categories: []string{"Q1", "Q2", "Q3", "Q4"},
			Series: []struct {
				Name   string
				Values []string
			}{
				{Name: "2025", Values: []string{"105200", "112400", "128600", "142800"}},
				{Name: "2026", Values: []string{"142600", "157000", "179800", "193800"}},
			},
		},
		{
			Title:      "Customer Growth",
			ChartType:  "line",
			Categories: []string{"Jan", "Feb", "Mar", "Apr", "May", "Jun"},
			Series: []struct {
				Name   string
				Values []string
			}{
				{Name: "Active Customers", Values: []string{"180", "188", "195", "204", "215", "228"}},
				{Name: "New Sign-ups", Values: []string{"12", "8", "15", "11", "14", "18"}},
			},
		},
	}

	if err := createTestXLSX(path, sheets, charts...); err != nil {
		t.Fatalf("failed to create example XLSX: %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Fatal("XLSX file was not created")
	}

	t.Logf("Generated %s", path)
}
