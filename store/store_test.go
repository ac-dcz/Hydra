package store

import "testing"

func TestNustDB(t *testing.T) {
	db := NewDefaultNutsDB("./0")
	store := NewStore(db)
	key, val := []byte("dcz"), []byte("1234")
	if err := store.Write(key, val); err != nil {
		t.Fatal(err)
	}
	if r_val, err := store.Read(key); err != nil {
		t.Fatal(err)
	} else if string(r_val) != string(val) {
		t.Fatalf("value is not equal\n")
	}
}
