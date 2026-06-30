package store

import (
	"path/filepath"
	"testing"
)

func TestCatalogExportImportMerge(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()

	if _, err := st.CreateNAS(NAS{Name: "ap-1", IP: "10.0.0.1", Secret: "s1", Vendor: "Symbol"}); err != nil {
		t.Fatal(err)
	}
	if _, err := st.CreateClient(Client{MAC: "00:11:22:33:44:55", Username: "alice"}); err != nil {
		t.Fatal(err)
	}

	exported, err := st.ExportCatalog()
	if err != nil {
		t.Fatal(err)
	}
	if len(exported.NAS) != 1 || len(exported.Clients) != 1 {
		t.Fatalf("unexpected export: %+v", exported)
	}

	result, err := st.ImportCatalog(CatalogImportRequest{
		Mode: "merge",
		NAS: []CatalogNAS{{
			Name:   "ap-1-updated",
			IP:     "10.0.0.1",
			Secret: "s2",
			Vendor: "Symbol",
		}},
		Clients: []CatalogClient{{
			MAC:      "00-11-22-33-44-55",
			Username: "bob",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.NASUpdated != 1 || result.ClientsUpdated != 1 {
		t.Fatalf("unexpected merge result: %+v", result)
	}

	nasList, err := st.ListNAS()
	if err != nil {
		t.Fatal(err)
	}
	if nasList[0].Name != "ap-1-updated" || nasList[0].Secret != "s2" {
		t.Fatalf("nas not updated: %+v", nasList[0])
	}

	clients, err := st.ListClients()
	if err != nil {
		t.Fatal(err)
	}
	if clients[0].Username != "bob" {
		t.Fatalf("client not updated: %+v", clients[0])
	}
}

func TestCatalogImportReplace(t *testing.T) {
	st := openTestStore(t)
	defer st.Close()

	if _, err := st.CreateNAS(NAS{Name: "old", IP: "10.0.0.9"}); err != nil {
		t.Fatal(err)
	}

	result, err := st.ImportCatalog(CatalogImportRequest{
		Mode: "replace",
		NAS: []CatalogNAS{{
			Name: "new-ap",
			IP:   "10.0.0.2",
		}},
		Clients: []CatalogClient{{
			MAC:      "aa:bb:cc:dd:ee:ff",
			Username: "u1",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if result.NASCreated != 1 || result.ClientsCreated != 1 {
		t.Fatalf("unexpected replace result: %+v", result)
	}

	nasList, err := st.ListNAS()
	if err != nil {
		t.Fatal(err)
	}
	if len(nasList) != 1 || nasList[0].IP != "10.0.0.2" {
		t.Fatalf("unexpected nas after replace: %+v", nasList)
	}
}

func openTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	return st
}
