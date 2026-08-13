package catalog

import "testing"

func TestCatalogNASName(t *testing.T) {
	if got := CatalogNASName(true, "IZB-DMD-WH", "IZB-DMD-069-AP310-1-C83C6A"); got != "PROXY-IZB-DMD-WH" {
		t.Fatalf("proxy rfd: got %q", got)
	}
	if got := CatalogNASName(true, "", "ap-1"); got != "" {
		t.Fatalf("proxy without rfd should not use AP name, got %q", got)
	}
	if got := CatalogNASName(false, "IZB-DMD-WH", "ap-1"); got != "ap-1" {
		t.Fatalf("no proxy: got %q", got)
	}
}

func TestMergeAutofillName(t *testing.T) {
	cases := []struct {
		existing, incoming, ip, want string
	}{
		{"", "PROXY-IZB-DMD-WH", "10.0.0.1", "PROXY-IZB-DMD-WH"},
		{"10.0.0.1", "ap-1", "10.0.0.1", "ap-1"},
		{"IZB-DMD-069-AP310-1-C83C6A", "PROXY-IZB-DMD-WH", "10.0.0.1", "PROXY-IZB-DMD-WH"},
		{"PROXY-IZB-DMD-WH", "PROXY-IZB-DMD-WH", "10.0.0.1", "PROXY-IZB-DMD-WH"},
		{"PROXY-IZB-DMD-WH", "PROXY-IZB-SPB", "10.0.0.1", "PROXY-VX"},
		{"PROXY-VX", "PROXY-IZB-DMD-WH", "10.0.0.1", "PROXY-VX"},
		{"PROXY-IZB-DMD-WH", "ap-1", "10.0.0.1", "PROXY-IZB-DMD-WH"},
		{"PROXY-IZB-DMD-WH", "", "10.0.0.1", "PROXY-IZB-DMD-WH"},
		{"custom-lab", "ap-1", "10.0.0.1", "custom-lab"},
	}
	for _, tc := range cases {
		got := MergeAutofillName(tc.existing, tc.incoming, tc.ip)
		if got != tc.want {
			t.Fatalf("merge(%q, %q) = %q, want %q", tc.existing, tc.incoming, got, tc.want)
		}
	}
}
