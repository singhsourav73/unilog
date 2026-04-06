package unilog

import "testing"

func TestParseOverflowPolicy(t *testing.T) {
	tests := []struct {
		in      string
		want    OverflowPolicy
		wantErr bool
	}{
		{in: "", want: OverflowDropNewest},
		{in: "drop_newest", want: OverflowDropNewest},
		{in: "drop-newest", want: OverflowDropNewest},
		{in: "block", want: OverflowBlock},
		{in: "unknown", wantErr: true},
	}

	for _, tt := range tests {
		got, err := ParseOverflowPolicy(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Fatalf("ParseOverflowPolicy(%q) expected error, got nil", tt.in)
			}
			continue
		}
		if err != nil {
			t.Fatalf("ParseOverflowPolicy(%q) unexpected error: %v", tt.in, err)
		}
		if got != tt.want {
			t.Fatalf("ParseOverflowPolicy(%q) = %v, want %v", tt.in, got, tt.want)
		}
	}
}
