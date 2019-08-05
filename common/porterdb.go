package common

import (
	"encoding/json"
	"fmt"

	"time"

	"github.com/saveio/porter/store"
	"github.com/saveio/themis/common/log"
	"github.com/syndtr/goleveldb/leveldb"
)

type PorterDB struct {
	db *store.LevelDBStore
}

func OpenPorterDB(file string) *PorterDB {
	if leveldb, err := store.NewLevelDBStore(file); err == nil {
		return NewPorterDB(leveldb)
	} else {
		log.Error("open porter leveldb error:", err.Error())
		return nil
	}
}

func NewPorterDB(d *store.LevelDBStore) *PorterDB {
	p := &PorterDB{
		db: d,
	}
	return p
}

func (this *PorterDB) Close() {
	this.db.Close()
}

func (this *PorterDB) InsertOrUpdatePort(connectionID string, port uint16, protocol string, timestamp time.Time) error {
	key := []byte(fmt.Sprintf("%s-%s", protocol, connectionID))
	info := &UsingPort{
		ConnectionID: connectionID,
		Port:         port,
		Protocol:     protocol,
		Timestamp:    timestamp}
	buf, err := json.Marshal(info)
	if err != nil {
		return err
	}
	return this.db.Put(key, buf)
}

func (this *PorterDB) QueryPort(connectionID string, protocol string) (*UsingPort, error) {
	key := []byte(fmt.Sprintf("%s-%s", protocol, connectionID))
	value, err := this.db.Get(key)
	if err != nil {
		if err != leveldb.ErrNotFound {
			return nil, err
		}
	}
	if len(value) == 0 {
		return nil, nil
	}

	info := &UsingPort{}
	err = json.Unmarshal(value, info)
	if err != nil {
		return nil, err
	}
	return info, nil
}

func (this *PorterDB) DeletePort(connectionID string, protocol string) error {
	key := []byte(fmt.Sprintf("%s-%s", protocol, connectionID))
	return this.db.Delete(key)
}
