nogo facts-only dependencies
============================

The fixture checks that packages outside ``nogo`` includes still produce facts
for checked dependents, without leaking diagnostics of their own. It covers
both a direct function fact and a method fact on a type re-exported through an
intermediate dependency; the latter must survive two facts-only compilations.
It also checks that a ``no-nogo`` relay can consume an analyzed dependency's
type export without trying to decode the dependency's analyzer facts.
The checked consumer calls a marked relay function: this must not report a fact
that would have been produced if analyzers had run on the relay.
