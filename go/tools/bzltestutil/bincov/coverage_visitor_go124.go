//go:build go1.24
// +build go1.24

package bincov

func emitTextual(d *dstate) error {
	return d.format.EmitTextual(nil, d.w)
}
