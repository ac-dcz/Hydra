package store

import (
	"WuKong/logger"

	"github.com/nutsdb/nutsdb"
)

type NutsDB struct {
	db *nutsdb.DB
}

const DefaultBucket = "DefaultBucket"

func NewDefaultNutsDB(dir string) *NutsDB {
	db, err := nutsdb.Open(nutsdb.DefaultOptions, nutsdb.WithDir(dir))
	if err != nil {
		logger.Error.Println(err)
		panic(err)
	}
	if err := db.Update(
		func(tx *nutsdb.Tx) error {
			return tx.NewBucket(nutsdb.DataStructureBTree, DefaultBucket)
		}); err != nil {
		logger.Error.Println(err)
		panic(err)
	}
	return &NutsDB{
		db: db,
	}
}

func (nuts *NutsDB) Get(key []byte) (val []byte, err error) {
	nuts.db.View(func(tx *nutsdb.Tx) error {
		val, err = tx.Get(DefaultBucket, key)
		return err
	})

	return
}

func (nuts *NutsDB) Put(key, val []byte) error {
	return nuts.db.Update(func(tx *nutsdb.Tx) error {
		if err := tx.Put(DefaultBucket, key, val, 0); err != nil {
			return err
		} else {
			return nil
		}
	})
}
