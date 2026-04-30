package balance

import "testing"

func TestCalculateReflexProcChance(t *testing.T) {
	tests := []struct {
		name   string
		reflex int
		want   int
	}{
		{name: "zero", reflex: 0, want: 0},
		{name: "five", reflex: 5, want: 10},
		{name: "eight", reflex: 8, want: 13},
		{name: "ten", reflex: 10, want: 15},
		{name: "fifteen", reflex: 15, want: 18},
		{name: "twenty", reflex: 20, want: 20},
		{name: "thirty", reflex: 30, want: 22},
		{name: "fifty capped", reflex: 50, want: 25},
		{name: "one hundred capped", reflex: 100, want: 25},
		{name: "negative", reflex: -5, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := CalculateReflexProcChance(tt.reflex)
			if got != tt.want {
				t.Fatalf("CalculateReflexProcChance(%d) = %d, want %d", tt.reflex, got, tt.want)
			}
		})
	}
}
