package bundle

// checkIDs gives every check a stable id, separate from the sentence shown to
// a person. The ids are the contract with the TypeScript verifier — the shared
// bundle vectors key on them — so a check can be reworded in either language
// without breaking anything, and the two verifiers are compared check by
// check rather than by matching English. web/app/lib/verify.ts holds the same
// ids; TestEveryCheckHasAnID fails if a check is added without one.
var checkIDs = map[string]string{
	"roster commitment binds root and size":                      "roster.commitment",
	"roster commitment belongs to this draw":                     "roster.draw",
	"draw parameters name this chain":                            "params.chain",
	"operator signature over draw parameters":                    "sig.params",
	"operator signature over roster commitment":                  "sig.roster",
	"parameters fixed before the cutoff":                         "order.params-before-cutoff",
	"roster frozen at or after the cutoff":                       "order.roster-after-cutoff",
	"roster frozen before the beacon round existed":              "order.roster-before-round",
	"target round leaves the minimum lead after cutoff":          "order.min-lead",
	"beacon round is the committed target":                       "beacon.target",
	"drand signature verifies against chain public key":          "beacon.signature",
	"seed derives from roster and beacon":                        "derive.seed",
	"winners derive from the seed":                               "derive.winners",
	"published leaves match the committed entry count":           "leaves.count",
	"published leaves rebuild to the committed root":             "leaves.root",
	"published entries match the committed entry count":          "entries.count",
	"published entries hash to the published leaves":             "entries.leaves",
	"private disclosure withholds the entry list":                "entries.withheld",
	"disclosure mode is recognised":                              "disclosure.mode",
	"each winner's leaf sits at its claimed index":               "winners.at-index",
	"each winner is proven to be in the committed roster":        "winners.proven",
	"log records form an unbroken chain":                         "log.chain",
	"log contains this draw's creation, cutoff and result":       "log.covers",
	"this draw was not abandoned and retried":                    "log.not-retried",
	"log head published to a log the organiser does not control": "anchor.present",
	"rekor inclusion proof verifies":                             "anchor.inclusion",
	"anchor binds a head from this draw's log":                   "anchor.binds",
	"checkpoint parses":                                          "anchor.checkpoint",
	"log head witnessed before the beacon round":                 "anchor.witnessed",
}
