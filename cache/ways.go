package cache

import (
	"github.com/jmhodges/levigo"
	"goposm/cache/binary"
	"goposm/element"
)

type WaysCache struct {
	Cache
}

func NewWaysCache(path string) (*WaysCache, error) {
	cache := WaysCache{}
	cache.options = &osmCacheOptions.Ways
	err := cache.open(path)
	if err != nil {
		return nil, err
	}
	return &cache, err
}

func (p *WaysCache) PutWay(way *element.Way) error {
	keyBuf := idToKeyBuf(way.Id)
	data, err := binary.MarshalWay(way)
	if err != nil {
		return err
	}
	return p.db.Put(p.wo, keyBuf, data)
}

func (p *WaysCache) PutWays(ways []element.Way) error {
	batch := levigo.NewWriteBatch()
	defer batch.Close()

	for _, way := range ways {
		keyBuf := idToKeyBuf(way.Id)
		data, err := binary.MarshalWay(&way)
		if err != nil {
			return err
		}
		batch.Put(keyBuf, data)
	}
	return p.db.Write(p.wo, batch)
}

func (p *WaysCache) GetWay(id int64) (*element.Way, error) {
	keyBuf := idToKeyBuf(id)
	data, err := p.db.Get(p.ro, keyBuf)
	if err != nil {
		return nil, err
	}
	if data == nil {
		return nil, NotFound
	}
	way, err := binary.UnmarshalWay(data)
	if err != nil {
		return nil, err
	}
	way.Id = id
	return way, nil
}

func (p *WaysCache) Iter() chan RawItem {
	ways := make(chan RawItem, 1024)
	go func() {
		ro := levigo.NewReadOptions()
		ro.SetFillCache(false)
		it := p.db.NewIterator(ro)
		defer it.Close()
		it.SeekToFirst()
		for ; it.Valid(); it.Next() {
			ways <- RawItem{idFromKeyBuf(it.Key()), it.Value()}
		}
		close(ways)
	}()
	return ways
}

func (self *WaysCache) FillMembers(members []element.Member) error {
	if members == nil || len(members) == 0 {
		return nil
	}
	for i, member := range members {
		if member.Type != element.WAY {
			continue
		}
		way, err := self.GetWay(member.Id)
		if err != nil {
			return err
		}
		members[i].Way = way
	}
	return nil
}

type InsertedWaysCache struct {
	Cache
}

func NewInsertedWaysCache(path string) (*InsertedWaysCache, error) {
	cache := InsertedWaysCache{}
	cache.options = &osmCacheOptions.InsertedWays

	err := cache.open(path)
	if err != nil {
		return nil, err
	}
	return &cache, err
}

func (p *InsertedWaysCache) PutMembers(members []element.Member) error {
	batch := levigo.NewWriteBatch()
	defer batch.Close()

	for _, m := range members {
		if m.Type != element.WAY {
			continue
		}
		keyBuf := idToKeyBuf(m.Id)
		batch.Put(keyBuf, []byte{})
	}
	return p.db.Write(p.wo, batch)
}

func (p *InsertedWaysCache) IsInserted(id int64) (bool, error) {
	keyBuf := idToKeyBuf(id)
	data, err := p.db.Get(p.ro, keyBuf)
	if err != nil {
		return false, err
	}
	if data == nil {
		return false, nil
	}
	return true, nil
}
