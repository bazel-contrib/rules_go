"""Settings that a transition sets to skip nogo for a Go subgraph.

Set SKIP_NOGO to True, and record the caller's value, JSON-encoded, in
ORIGINAL_SKIP_NOGO when that is unset; non_go_transition restores it on
non-Go edges, so non-Go dependencies keep the caller's configuration.
"""

SKIP_NOGO = str(Label("//go/private:bootstrap_nogo"))

ORIGINAL_SKIP_NOGO = str(Label("//go/private:original_bootstrap_nogo"))
