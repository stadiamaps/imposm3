package writer

import (
	"goposm/cache"
	"goposm/element"
	"goposm/geom"
	"goposm/geom/geos"
	"goposm/mapping"
	"goposm/proj"
	"goposm/stats"
	"log"
	"sync"
)

type WayWriter struct {
	OsmElemWriter
	ways                 chan *element.Way
	lineStringTagMatcher *mapping.TagMatcher
	polygonTagMatcher    *mapping.TagMatcher
}

func NewWayWriter(osmCache *cache.OSMCache, ways chan *element.Way,
	insertBuffer *InsertBuffer, lineStringTagMatcher *mapping.TagMatcher,
	polygonTagMatcher *mapping.TagMatcher, progress *stats.Statistics) *OsmElemWriter {
	ww := WayWriter{
		OsmElemWriter: OsmElemWriter{
			osmCache:     osmCache,
			progress:     progress,
			wg:           &sync.WaitGroup{},
			insertBuffer: insertBuffer,
		},
		ways:                 ways,
		lineStringTagMatcher: lineStringTagMatcher,
		polygonTagMatcher:    polygonTagMatcher,
	}
	ww.OsmElemWriter.writer = &ww
	return &ww.OsmElemWriter
}

func (ww *WayWriter) loop() {
	geos := geos.NewGeos()
	defer geos.Finish()
	for w := range ww.ways {
		ww.progress.AddWays(1)
		inserted, err := ww.osmCache.InsertedWays.IsInserted(w.Id)
		if err != nil {
			log.Println(err)
			continue
		}
		if inserted {
			continue
		}

		err = ww.osmCache.Coords.FillWay(w)
		if err != nil {
			continue
		}
		proj.NodesToMerc(w.Nodes)
		if matches := ww.lineStringTagMatcher.Match(&w.Tags); len(matches) > 0 {
			ww.buildAndInsert(geos, w, matches, geom.LineStringWkb)
		}
		if w.IsClosed() {
			if matches := ww.polygonTagMatcher.Match(&w.Tags); len(matches) > 0 {
				ww.buildAndInsert(geos, w, matches, geom.PolygonWkb)
			}
		}

		// if *diff {
		// 	ww.diffCache.Coords.AddFromWay(w)
		// }
	}
	ww.wg.Done()
}

type geomBuilder func(*geos.Geos, []element.Node) (*element.Geometry, error)

func (ww *WayWriter) buildAndInsert(geos *geos.Geos, w *element.Way, matches []mapping.Match, builder geomBuilder) {
	var err error
	// make copy to avoid interference with polygon/linestring matches
	way := element.Way(*w)
	way.Geom, err = builder(geos, way.Nodes)
	if err != nil {
		if err, ok := err.(ErrorLevel); ok {
			if err.Level() <= 0 {
				return
			}
		}
		log.Println(err)
		return
	}
	if ww.clipper != nil {
		parts, err := ww.clipper.Clip(way.Geom.Geom)
		if err != nil {
			log.Println(err)
			return
		}
		for _, g := range parts {
			way := element.Way(*w)
			way.Geom = &element.Geometry{g, geos.AsWkb(g)}
			ww.insertMatches(&way.OSMElem, matches)
		}
	} else {
		ww.insertMatches(&way.OSMElem, matches)
	}
}