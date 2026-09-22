/** Wire types, mirroring the Go JSON shapes. Hex strings stay strings here. */

export type Disclosure = 'public' | 'private'
export type DrawState = 'open' | 'committed' | 'drawn'

export interface Entry {
  label: string
}

export interface Sealed {
  /** hex */
  salt: string
  entry: Entry
}

export interface DrawParams {
  draw_id: string
  title: string
  winners: number
  disclosure: Disclosure
  cutoff_at: number
  target_round: number
  chain_hash: string
  created_at: number
}

export interface RosterCommitment {
  draw_id: string
  root: string
  size: number
  commit: string
  committed_at: number
}

export interface BeaconRound {
  round: number
  signature: string
}

export interface Winner {
  rank: number
  index: number
  salt: string
  entry: Entry
  merkle_path: string[]
}

export interface LogRecord {
  seq: number
  kind: 'draw_created' | 'roster_committed' | 'result_published'
  draw_id: string
  payload: string
  logged_at: number
  prev: string
  head: string
}

export interface Admission {
  index: number
  salt: string
  label: string
}

export interface Bundle {
  version: number
  operator_pubkey: string
  draw: { params: DrawParams; signature: string }
  roster: {
    commitment: RosterCommitment
    signature: string
    /**
     * Every committed leaf, in order, published in both disclosure modes.
     * Salted, so it discloses nothing to anyone who does not already hold the
     * salt — and it is what lets an entrant confirm their own inclusion from
     * the salt they were given at entry, with no receipt needing to arrive.
     */
    leaves: string[]
    entries?: Sealed[]
    /**
     * Publishes the transparency-log head into Sigstore's Rekor, a public
     * append-only log the organiser does not run. Absent on draws that were
     * never anchored, in which case verification says so rather than passing.
     */
    rekor_anchor?: import('./rekor').Anchor
    ots_proof?: string
  }
  beacon: BeaconRound
  result: { seed: string; winners: Winner[]; waitlist: number[] }
  log: { log_id: string; records: LogRecord[]; head: string }
}

/** The public half of a receipt: no salt, no label. */
export interface Proof {
  draw_id: string
  root: string
  size: number
  index: number
  merkle_path: string[]
}

export interface Receipt {
  draw_id: string
  root: string
  size: number
  index: number
  salt: string
  entry: Entry
  merkle_path: string[]
}

export type Visibility = 'public' | 'unlisted'

export interface DrawSummary {
  params: DrawParams
  params_sig: string
  state: DrawState
  entry_count: number
  roster_commit?: RosterCommitment
  roster_sig?: string
  visibility?: Visibility
  labels?: string[]
}
