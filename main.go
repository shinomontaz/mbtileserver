package main

import (
	"bytes"
	"compress/gzip"
	"database/sql"
	"errors"
	"fmt"
	"image"
	"io/ioutil"
	"log"
	"net/http"
	"regexp"
	"strconv"

	"net/http/pprof"
	_ "net/http/pprof"

	"github.com/gorilla/mux"
	_ "github.com/mattn/go-sqlite3"
)

var (
	db                 *sql.DB
	tileSelectStmt     *sql.Stmt
	metadataSelectStmt *sql.Stmt
	err                error
	tileData           []byte
	tileDecoded        []byte
	img                image.Image
	zxyRegexp          *regexp.Regexp
	port               string
	dsn                string
)

func init() {
	dsn = "../russia.mbtiles"
	port = ":3001"

	db, err = sql.Open("sqlite3", dsn)
	if err != nil {
		log.Fatal("cannot open DB: ", err)
	}

	tileSelectStmt, err = db.Prepare("SELECT tile_data FROM tiles WHERE zoom_level = ? AND tile_column = ? AND tile_row = ?;")
	if err != nil {
		log.Fatal("cannot prepare tile data statement: ", err)
	}
	metadataSelectStmt, err = db.Prepare("SELECT value FROM metadata WHERE name = ?;")
	if err != nil {
		log.Fatal("cannot prepare metadata statement: ", err)
	}
	zxyRegexp = regexp.MustCompile(`\A([0-9]+)/([0-9]+)/([0-9]+)\z`)
}

func main() {
	defer func() {
		if err := closeTiles(); err != nil {
			log.Print(err)
		}
	}()

	r := mux.NewRouter()
	r.Handle("/debug/pprof/", http.HandlerFunc(pprof.Index))
	r.Handle("/debug/pprof/cmdline", http.HandlerFunc(pprof.Cmdline))
	r.Handle("/debug/pprof/profile", http.HandlerFunc(pprof.Profile))
	r.Handle("/debug/pprof/symbol", http.HandlerFunc(pprof.Symbol))
	r.Handle("/debug/pprof/trace", http.HandlerFunc(pprof.Trace))
	r.Handle("/debug/pprof/{cmd}", http.HandlerFunc(pprof.Index))
	r.HandleFunc("/tiles/{z}/{x}/{y}.png", handler)

	http.ListenAndServe(port, r)
}

func closeTiles() error {
	if tileSelectStmt != nil {
		if err2 := tileSelectStmt.Close(); err2 != nil {
			err = err2
		}
	}
	if db != nil {
		if err2 := db.Close(); err2 != nil {
			err = err2
		}
	}

	return err
}

type TileFormat uint8

const (
	UNKNOWN TileFormat = iota
	GZIP               // encoding = gzip
	ZLIB               // encoding = deflate
	PNG
	JPG
	PBF
	WEBP
)

func detectTileFormat(data *[]byte) (TileFormat, error) {
	patterns := map[TileFormat][]byte{
		GZIP: []byte("\x1f\x8b"), // this masks PBF format too
		ZLIB: []byte("\x78\x9c"),
		PNG:  []byte("\x89\x50\x4E\x47\x0D\x0A\x1A\x0A"),
		JPG:  []byte("\xFF\xD8\xFF"),
		WEBP: []byte("\x52\x49\x46\x46\xc0\x00\x00\x00\x57\x45\x42\x50\x56\x50"),
	}

	for format, pattern := range patterns {
		if bytes.HasPrefix(*data, pattern) {
			return format, nil
		}
	}

	return UNKNOWN, errors.New("Could not detect tile format")
}

func getTile(z, x, y int) ([]byte, error) {
	err := tileSelectStmt.QueryRow(z, x, 1<<uint(z)-y-1).Scan(&tileData)

	return tileData, err
}

func getMeta(name string) (string, error) {
	var value string
	err := metadataSelectStmt.QueryRow(name).Scan(&value)
	return value, err
}

func handler(w http.ResponseWriter, req *http.Request) {

	vars := mux.Vars(req)
	z, _ := strconv.Atoi(vars["z"])
	x, _ := strconv.Atoi(vars["x"])
	y, _ := strconv.Atoi(vars["y"])
	tileData, err := getTile(z, x, y)
	if err != nil {
		http.NotFound(w, req)
		return
	}

	format, err := detectTileFormat(&tileData)
	if format == GZIP {
		reader := bytes.NewReader(tileData)
		gzreader, err := gzip.NewReader(reader)
		if err != nil {
			fmt.Println("getTile error: ", err)
		}
		tileData, err = ioutil.ReadAll(gzreader)
		if err != nil {
			fmt.Println("getTile read error: ", err)
		}
	}

	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Content-Type", "image/png")
	w.Header().Add("Content-Length", strconv.Itoa(len(tileData)))

	_, _ = w.Write(tileData)
}
