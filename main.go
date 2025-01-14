package sfs

import (
	"errors"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	ErrMissingKey = errors.New("missing key - unable to save data")
)

// Logger is a generic logger,this is very simple store,should simplet system log.
type Logger interface {
	//Fatal(string, ...interface{})
	//Error(string, ...interface{})
	//Warn(string, ...interface{})
	//Info(string, ...interface{})
	//Debug(string, ...interface{})
	//Trace(string, ...interface{})
	Printf(format string, v ...any)
	Println(v ...any)
	Fatalf(format string, v ...any)
	Fatalln(v ...any)

}

// Driver holds the config and interacts with the underlying file store.
type Driver struct {
	mutex     sync.Mutex
	mutexes   map[string]*sync.Mutex
	baseDir   string
	log       Logger
	marshaler Marshaler
	placer    Placer
}

type Placer interface {
	GetPath(key string) string
}

type SimplePlacer struct{}

func (p SimplePlacer) GetPath(key string) string {
	return key
}

// Options for optional config
type Options struct {
	Logger
	Marshaler
	Placer
}

func New(baseDir string) (*Driver, error) {
	return NewWithOption(baseDir,&Options{})
}
// New creates a new driver at the given location, and returns a *Driver
// for further interaction. By default will use teh JSONMarshaler.
func NewWithOption(baseDir string, options *Options) (*Driver, error) {
	if options == nil {
		options = &Options{}
	}
	log.Println()
	if options.Logger == nil {
		//options.Logger = lumber.NewConsoleLogger(lumber.INFO)
		options.Logger = log.Default()
	}
	if options.Marshaler == nil {
		options.Marshaler = &JSONMarshaler{}
	}
	if options.Placer == nil {
		options.Placer = &SimplePlacer{}
	}
	baseDir = filepath.Clean(baseDir)

	driver := Driver{
		baseDir:   baseDir,
		mutexes:   make(map[string]*sync.Mutex),
		log:       options.Logger,
		marshaler: options.Marshaler,
		placer:    options.Placer,
	}
	if _, err := os.Stat(baseDir); err == nil {
		options.Logger.Printf("Using '%s' (folder already exists)\n", baseDir)
		return &driver, nil
	}

	options.Logger.Printf("Creating storage folder at '%s'...\n", baseDir)
	return &driver, os.MkdirAll(baseDir, 0755)
}

// Write the value [v] to the [key], using the
// marshaler.
func (d *Driver) Write(key string, v interface{}) error {

	if key == "" {
		//return ErrMissingKey
		key="main"
	}

	mutex := d.getOrCreateMutex(key)
	mutex.Lock()
	defer mutex.Unlock()

	return d.writeFile(key, v)
}

func (d *Driver) writeFile(key string, v interface{}) error {

	path := d.placer.GetPath(key)
	dir := filepath.Join(d.baseDir, filepath.Dir(path))
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := d.marshaler.Marshal(v)
	if err != nil {
		return err
	}

	file := filepath.Join(d.baseDir, path+d.marshaler.GetFileExtension())
	tmpFile := file + ".tmp"

	if err := ioutil.WriteFile(tmpFile, data, 0644); err != nil {
		return err
	}
	return os.Rename(tmpFile, file)
}

// Read the content from [key] into [v].
func (d *Driver) Read(key string, v interface{}) error {

	if key == "" {
		//return ErrMissingKey
		key="main"
	}

	return d.readFile(key, v)
}

func (d *Driver) readFile(key string, v interface{}) error {

	path := d.placer.GetPath(key)
	file := filepath.Join(d.baseDir, path+d.marshaler.GetFileExtension())

	data, err := ioutil.ReadFile(file)
	if err != nil {
		return err
	}

	return d.marshaler.Unmarshal(data, v)
}

func (d *Driver) getOrCreateMutex(parent string) *sync.Mutex {

	d.mutex.Lock()
	defer d.mutex.Unlock()

	m, ok := d.mutexes[parent]
	if !ok {
		m = &sync.Mutex{}
		d.mutexes[parent] = m
	}

	return m
}
