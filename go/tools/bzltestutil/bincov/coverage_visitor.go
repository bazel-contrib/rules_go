package bincov

// This is a stripped-down version of https://github.com/golang/go/blob/633dd1d475e7346b43d87abc987a8c7f256e827d/src/cmd/covdata/dump.go
// which supports only the `textfmt` sub-command.
// TODO(zbarsky): We should emit lcov format directly instead of going through the textual format.

import (
	"cmd/internal/cov"
	"fmt"
	"internal/coverage"
	"internal/coverage/calloc"
	"internal/coverage/cformat"
	"internal/coverage/cmerge"
	"internal/coverage/decodecounter"
	"internal/coverage/decodemeta"
	"internal/coverage/pods"
	"io"
	"os"
)

func makeVisitor(w io.Writer) cov.CovDataVisitor {
	return &dstate{
		w:  w,
		cm: &cmerge.Merger{},
	}
}

type pkfunc struct {
	pk, fcn uint32
}

func warn(s string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "warning: ")
	fmt.Fprintf(os.Stderr, s, a...)
	fmt.Fprintf(os.Stderr, "\n")
}

func fatal(s string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, "error: ")
	fmt.Fprintf(os.Stderr, s, a...)
	fmt.Fprintf(os.Stderr, "\n")
	os.Exit(1)
}

// dstate encapsulates state and provides methods for implementing
// various dump operations. Specifically, dstate implements the
// CovDataVisitor interface, and is designed to be used in
// concert with the CovDataReader utility, which abstracts away most
// of the grubby details of reading coverage data files.
type dstate struct {
	// for batch allocation of counter arrays
	calloc.BatchCounterAlloc

	// counter merging state + methods
	cm *cmerge.Merger

	// counter data formatting helper
	format *cformat.Formatter

	// 'mm' stores values read from a counter data file; the pkfunc key
	// is a pkgid/funcid pair that uniquely identifies a function in
	// instrumented application.
	mm map[pkfunc]decodecounter.FuncPayload

	// pkm maps package ID to the number of functions in the package
	// with that ID. It is used to report inconsistencies in counter
	// data (for example, a counter data entry with pkgid=N funcid=10
	// where package N only has 3 functions).
	pkm map[uint32]uint32

	// Writer to which we will write text format output
	w io.Writer
}

// Setup is called once at program startup time to vet flag values
// and do any necessary setup operations.
func (d *dstate) Setup() {
}

func (d *dstate) BeginPod(p pods.Pod) {
	d.mm = make(map[pkfunc]decodecounter.FuncPayload)
}

func (d *dstate) EndPod(p pods.Pod) {
}

func (d *dstate) BeginCounterDataFile(cdf string, cdr *decodecounter.CounterDataReader, dirIdx int) {
}

func (d *dstate) EndCounterDataFile(cdf string, cdr *decodecounter.CounterDataReader, dirIdx int) {
}

func (d *dstate) VisitFuncCounterData(data decodecounter.FuncPayload) {
	if nf, ok := d.pkm[data.PkgIdx]; !ok || data.FuncIdx > nf {
		warn("func payload inconsistency: id [p=%d,f=%d] nf=%d len(ctrs)=%d in VisitFuncCounterData, ignored", data.PkgIdx, data.FuncIdx, nf, len(data.Counters))
		return
	}
	key := pkfunc{pk: data.PkgIdx, fcn: data.FuncIdx}
	val := d.mm[key]

	if len(val.Counters) < len(data.Counters) {
		t := val.Counters
		val.Counters = d.AllocateCounters(len(data.Counters))
		copy(val.Counters, t)
	}
	err, overflow := d.cm.MergeCounters(val.Counters, data.Counters)
	if err != nil {
		fatal("%v", err)
	}
	if overflow {
		warn("uint32 overflow during counter merge")
	}
	d.mm[key] = val
}

func (d *dstate) EndCounters() {
}

func (d *dstate) VisitMetaDataFile(mdf string, mfr *decodemeta.CoverageMetaFileReader) {
	newgran := mfr.CounterGranularity()
	newmode := mfr.CounterMode()
	if err := d.cm.SetModeAndGranularity(mdf, newmode, newgran); err != nil {
		fatal("%v", err)
	}
	if d.format == nil {
		d.format = cformat.NewFormatter(mfr.CounterMode())
	}

	// To provide an additional layer of checking when reading counter
	// data, walk the meta-data file to determine the set of legal
	// package/function combinations. This will help catch bugs in the
	// counter file reader.
	d.pkm = make(map[uint32]uint32)
	np := uint32(mfr.NumPackages())
	payload := []byte{}
	for pkIdx := uint32(0); pkIdx < np; pkIdx++ {
		var pd *decodemeta.CoverageMetaDataDecoder
		var err error
		pd, payload, err = mfr.GetPackageDecoder(pkIdx, payload)
		if err != nil {
			fatal("reading pkg %d from meta-file %s: %s", pkIdx, mdf, err)
		}
		d.pkm[pkIdx] = pd.NumFuncs()
	}
}

func (d *dstate) BeginPackage(pd *decodemeta.CoverageMetaDataDecoder, pkgIdx uint32) {
	d.format.SetPackage(pd.PackagePath())
}

func (d *dstate) EndPackage(pd *decodemeta.CoverageMetaDataDecoder, pkgIdx uint32) {
}

func (d *dstate) VisitFunc(pkgIdx uint32, fnIdx uint32, fd *coverage.FuncDesc) {
	var counters []uint32
	key := pkfunc{pk: pkgIdx, fcn: fnIdx}
	v, haveCounters := d.mm[key]

	if haveCounters {
		counters = v.Counters
	}

	for i := 0; i < len(fd.Units); i++ {
		u := fd.Units[i]
		var count uint32
		if counters != nil {
			count = counters[i]
		}
		d.format.AddUnit(fd.Srcfile, fd.Funcname, fd.Lit, u, count)
	}
}

func (d *dstate) Finish() {
	// d.format maybe nil here if the specified input dir was empty.
	if d.format != nil {
		if err := emitTextual(d); err != nil {
			fatal("writing textual output: %v", err)
		}
	}
}
