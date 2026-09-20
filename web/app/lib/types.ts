/** Wire types, mirroring the Go JSON shapes. Hex strings stay strings here. */

export type Disclosure = 'public' | 'private'
export type DrawState = 'open' | 'committed' | 'drawn'

export interface Entry {
  label: string
}

export interface Sealed {
  /** base64 in the Go JSON encoding of []byte */
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

export interface Bundle {
  version: number
  operator_pubkey: string
  draw: { params: DrawParams; signature: string }
  roster: {
    commitment: RosterCommitment
    signature: string
    entries?: Sealed[]
    ots_proof?: string
  }
  beacon: BeaconRound
  result: { seed: string; winners: Winner[]; waitlist: number[] }
  log: { log_id: string; records: LogRecord[]; head: string }
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

export interface DrawSummary {
  params: DrawParams
  params_sig: string
  state: DrawState
  entry_count: number
  roster_commit?: RosterCommitment
  roster_sig?: string
  labels?: string[]
}
