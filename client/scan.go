package main

import (
	"crypto/md5"
	"errors"
	"fmt"
	"hash"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	bolt "go.etcd.io/bbolt"
)

const (
	BUCKET_TO_HASH   = "hash"
	BUCKET_FROM_HASH = "hash_reverse"
)

func CreateHashBucket(db *bolt.DB, clean bool) error {
	return db.Update(func(tx *bolt.Tx) error {
		if clean {
			b := tx.Bucket([]byte(BUCKET_TO_HASH))
			if b != nil {
				tx.DeleteBucket([]byte(BUCKET_TO_HASH))
			}
		}
		if clean {
			b := tx.Bucket([]byte(BUCKET_FROM_HASH))
			if b != nil {
				tx.DeleteBucket([]byte(BUCKET_FROM_HASH))
			}
		}

		_, err := tx.CreateBucketIfNotExists([]byte(BUCKET_TO_HASH))
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists([]byte(BUCKET_FROM_HASH))
		if err != nil {
			return err
		}

		return nil
	})
}

func ScanSongFolder(db *bolt.DB, folder string) error {
	slog.Info("scanning song folder...")

	packs, err := os.ReadDir(folder)
	if err != nil {
		panic(fmt.Errorf("os readdir (folder): %w", err))
	}

	for _, pack := range packs {
		if !pack.IsDir() {
			slog.Warn("folder is not a pack folder, ignoring...", slog.String("folder", pack.Name()))
			continue
		}
		if BlacklistedSongFolders[pack.Name()] {
			continue
		}

		songs, err := os.ReadDir(filepath.Join(folder, pack.Name()))
		if err != nil {
			panic(fmt.Errorf("os readdir (pack): %w", err))
		}

		for _, song := range songs {
			p := filepath.Join(folder, pack.Name(), song.Name())

			if !song.IsDir() || !HasSMFile(p) {
				slog.Warn("folder is not a song folder, ignoring...", slog.String("folder", song.Name()))
				continue
			}
			if BlacklistedSongFiles[pack.Name()] {
				continue
			}

			key := fmt.Sprintf("%s/%s", pack.Name(), song.Name())

			hash, err := HashSongFolder(p)
			if err != nil {
				panic(fmt.Errorf("hash: %w", err))
			}

			err = db.Update(func(tx *bolt.Tx) error {
				hB := tx.Bucket([]byte(BUCKET_TO_HASH))
				if err := hB.Put([]byte(key), hash); err != nil {
					return err
				}
				hBr := tx.Bucket([]byte(BUCKET_FROM_HASH))
				if err := hBr.Put(hash, []byte(key)); err != nil {
					return err
				}
				return nil
			})
			if err != nil {
				panic(fmt.Errorf("db: %w", err))
			}

			if Verbose {
				slog.Info("hashed file", slog.String("key", key), slog.String("hash", string(hash)))
			} else {
				slog.Info("hashed file", slog.String("key", key))
			}
		}
	}

	slog.Info("scanned song folder!")
	fmt.Println()
	return nil
}

func IsValidSongDirectory(dir string) (bool, error) {
	// Check if we have a "pack" folder
	packs, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return false, nil
		}

		return false, err
	}

	var pack string
	for _, p := range packs {
		if !p.IsDir() {
			continue
		}

		pack = p.Name()
		break
	}
	if pack == "" {
		return false, nil
	}

	files, err := os.ReadDir(filepath.Join(dir, pack))
	if err != nil {
		return false, err
	}

	for _, f := range files {
		if !f.IsDir() {
			continue
		}

		if !HasSMFile(filepath.Join(dir, pack, f.Name())) {
			continue
		}

		return true, nil
	}

	return false, nil
}

func HasSMFile(dir string) bool {
	files, err := os.ReadDir(dir)
	if err != nil {
		return false
	}

	for _, file := range files {
		if file.IsDir() {
			continue
		}

		if filepath.Ext(file.Name()) == ".sm" {
			return true
		}
	}

	return false
}
func CanHashExtension(ext string) bool {
	return ext == ".xml" ||
		ext == ".lua" ||
		ext == ".sm" ||
		ext == ".ini"
}

func HashSongFolder(f string) ([]byte, error) {
	hi := &FolderHashInstance{
		Hash: md5.New(),
	}

	if err := filepath.WalkDir(f, hi.Walk); err != nil {
		return nil, err
	}

	return fmt.Appendf(nil, "%x", hi.Hash.Sum(nil)), nil
}

type FolderHashInstance struct {
	Hash hash.Hash
}

func (i *FolderHashInstance) Walk(path string, d fs.DirEntry, err error) error {
	if d.IsDir() && d.Name() == ".git" {
		return filepath.SkipDir
	}

	if d.IsDir() {
		return nil
	}

	ext := filepath.Ext(d.Name())
	if !CanHashExtension(ext) {
		// fmt.Println("ignoring extension " + ext + "...")
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	_, err = i.Hash.Write(data)
	if err != nil {
		return err
	}

	return nil
}

func HasSongKey(db *bolt.DB, key string) bool {
	has := false

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BUCKET_TO_HASH))
		has = b.Get([]byte(key)) != nil
		return nil
	})
	if err != nil {
		return false
	}

	return has
}
func HasSongHash(db *bolt.DB, hash string) bool {
	has := false

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BUCKET_FROM_HASH))
		has = b.Get([]byte(hash)) != nil
		return nil
	})
	if err != nil {
		return false
	}

	return has
}

func GetSongHash(db *bolt.DB, key string) (string, bool) {
	hash := ""
	has := false

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BUCKET_TO_HASH))
		if data := b.Get([]byte(key)); data != nil {
			hash = string(data)
			has = true
		}
		return nil
	})
	if err != nil {
		return "", false
	}

	return hash, has
}
func GetSongKey(db *bolt.DB, hash string) (string, bool) {
	key := ""
	has := false

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BUCKET_FROM_HASH))

		if data := b.Get([]byte(hash)); data != nil {
			key = string(data)
			has = true
		}
		return nil
	})
	if err != nil {
		return "", false
	}

	return key, has
}

func CountSongs(db *bolt.DB) int {
	count := 0

	err := db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte(BUCKET_TO_HASH))
		b.ForEach(func(k, v []byte) error {
			if !strings.Contains(string(k), "/") {
				return nil
			}

			count += 1
			return nil
		})
		return nil
	})
	if err != nil {
		return -1
	}

	return count
}
