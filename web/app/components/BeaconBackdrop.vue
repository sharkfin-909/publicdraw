<script setup lang="ts">
/**
 * A die tumbling in the left margin, with rings spreading out from it — the
 * draw's randomness, broadcast. Pure CSS: 3D-transformed planes and SVG rings,
 * so there is no WebGL and nothing to download.
 *
 * The die is built as a real rounded cube rather than six rounded cards, which
 * leave the corners hollow when seen side on:
 *   - six flat faces, inset from the edge by the rounding radius;
 *   - twelve rounded edges, each a quarter-cylinder of three narrow strips;
 *   - eight corners, each a sphere. A sphere's outline is a circle from any
 *     angle, so it is a disc that counter-rotates to always face the viewer.
 *
 * Decorative only: fixed behind the page, ignores the pointer, hidden from
 * assistive tech, dropped where the margin is too narrow to hold it, and held
 * still for anyone who has asked for reduced motion.
 */

const S = 52            // edge length
const R = 7             // rounding radius
const H = S / 2
const FLAT = S - 2 * R  // flat part of each face

// Pips on a 3x3 grid, cells numbered 0–8. Opposite faces sum to seven.
const faces = [
  { n: 1, turn: 'rotateY(0deg)',   pips: [4] },
  { n: 6, turn: 'rotateY(180deg)', pips: [0, 2, 3, 5, 6, 8] },
  { n: 2, turn: 'rotateY(90deg)',  pips: [0, 8] },
  { n: 5, turn: 'rotateY(-90deg)', pips: [0, 2, 4, 6, 8] },
  { n: 3, turn: 'rotateX(90deg)',  pips: [0, 4, 8] },
  { n: 4, turn: 'rotateX(-90deg)', pips: [0, 2, 6, 8] },
].map(f => ({ ...f, transform: `${f.turn} translateZ(${H}px)` }))

// Each edge is built in one frame — parallel to Y, between the +x and +z
// faces — then turned into place: four quadrants about Y, and that set turned
// onto X and onto Z for the other eight.
const STRIPS = 3
const step = 90 / STRIPS
const rad = (d: number) => (d * Math.PI) / 180
const chord = 2 * R * Math.sin(rad(step / 2)) + 0.6 // a hair wide, so no seams
const apothem = R * Math.cos(rad(step / 2))

const edges = ['', 'rotateZ(90deg) ', 'rotateX(90deg) '].flatMap((axis, a) =>
  [0, 1, 2, 3].flatMap(q =>
    Array.from({ length: STRIPS }, (_, i) => {
      const angle = step / 2 + i * step
      return {
        key: `${a}-${q}-${i}`,
        // The middle strip faces furthest from both faces, so is the darkest.
        shade: i === Math.floor(STRIPS / 2) ? 'mid' : 'side',
        transform:
          `${axis}rotateY(${q * 90}deg) translate3d(${H - R}px, 0, ${H - R}px) ` +
          `rotateY(${angle}deg) translateZ(${apothem}px)`,
      }
    }),
  ),
)

const c = H - R
const corners = [-1, 1].flatMap(x => [-1, 1].flatMap(y => [-1, 1].map(z => ({
  key: `${x}${y}${z}`,
  transform: `translate3d(${x * c}px, ${y * c}px, ${z * c}px)`,
}))))

// The crisp edge: an ink wire along the crest of each rounded edge, carried
// across each corner sphere to the corner's tip, where three wires meet. Each
// segment is two thin ribbons crossed at right angles, so the line still shows
// when its edge is on the silhouette and a single ribbon would be seen edge-on.
type V = [number, number, number]
const sub = (p: V, q: V): V => [p[0] - q[0], p[1] - q[1], p[2] - q[2]]
const dot = (p: V, q: V) => p[0] * q[0] + p[1] * q[1] + p[2] * q[2]
const scale = (p: V, k: number): V => [p[0] * k, p[1] * k, p[2] * k]
const unit = (p: V) => scale(p, 1 / Math.hypot(...p))
const cross = (p: V, q: V): V =>
  [p[1] * q[2] - p[2] * q[1], p[2] * q[0] - p[0] * q[2], p[0] * q[1] - p[1] * q[0]]

const WIRE = 0.5
// Lifted ~0.35px off the surface, or the wire and the edge strips fight over
// which is in front and the line breaks into dashes.
const LIFT = 0.35
const crestOff = c + (R + LIFT) / Math.SQRT2
const tipOff = c + (R + LIFT) / Math.sqrt(3)

function ribbons(from: V, to: V, out: V, key: string) {
  const d = sub(to, from)
  const length = Math.hypot(...d) + 0.6 // overlap at the joints
  const x = unit(d)
  const z = unit(sub(out, scale(x, dot(out, x))))
  const y = cross(z, x)
  const mid = scale([from[0] + to[0], from[1] + to[1], from[2] + to[2]], 0.5)
  const m = (X: V, Y: V, Z: V) =>
    `matrix3d(${[...X, 0, ...Y, 0, ...Z, 0, ...mid, 1].map(n => +n.toFixed(4)).join(',')})`
  const at = box(length, WIRE)
  return [
    { key: `${key}a`, style: { ...at, transform: m(x, y, z) } },
    { key: `${key}b`, style: { ...at, transform: m(x, scale(z, -1), y) } },
  ]
}

const wires = [0, 1, 2].flatMap(axis => {
  const [i, j] = [0, 1, 2].filter(n => n !== axis) as [number, number]
  return [-1, 1].flatMap(si => [-1, 1].flatMap(sj => {
    const point = (oi: number, oj: number, t: number): V => {
      const p: V = [0, 0, 0]; p[i] = si * oi; p[j] = sj * oj; p[axis] = t; return p
    }
    const crestOut = point(1, 1, 0)
    const tip = (end: number) => point(tipOff, tipOff, end * tipOff)
    const crest = (end: number) => point(crestOff, crestOff, end * c)
    const sphere = (end: number) => point(c, c, end * c)
    const key = `${axis}${si}${sj}`
    return [
      ...ribbons(crest(-1), crest(1), crestOut, `${key}e`),
      ...[-1, 1].flatMap(end => {
        const from = crest(end), to = tip(end)
        const mid = scale([from[0] + to[0], from[1] + to[1], from[2] + to[2]], 0.5)
        return ribbons(from, to, sub(mid, sphere(end)), `${key}c${end}`)
      }),
    ]
  }))
})

// A function declaration, so it is hoisted: the wires above are built with it.
function box(w: number, h: number) {
  return {
    width: `${w}px`, height: `${h}px`,
    left: `${(S - w) / 2}px`, top: `${(S - h) / 2}px`,
  }
}
const faceBox = box(FLAT + 0.5, FLAT + 0.5)
const stripBox = box(chord, FLAT + 0.5)
// Sized to the strips' inner radius, not R, or the corners bulge past the
// faceted edges when seen edge-on.
const ball = 2 * apothem + 0.2
const ballBox = box(ball, ball)

const rings = 4
</script>

<template>
  <div class="backdrop" aria-hidden="true">
    <div class="origin">
      <svg class="rings" viewBox="-500 -500 1000 1000">
        <circle
          v-for="i in rings" :key="i"
          r="500" class="ring" :style="{ animationDelay: `${-(i - 1) * (12 / rings)}s` }"
          vector-effect="non-scaling-stroke"
        />
      </svg>
      <div class="stage" :style="{ width: `${S}px`, height: `${S}px`, left: `${-H}px`, top: `${-H}px` }">
        <div class="die">
          <div
            v-for="face in faces" :key="face.n"
            class="face" :style="{ ...faceBox, transform: face.transform }"
          >
            <span v-for="cell in 9" :key="cell" class="cell">
              <i v-if="face.pips.includes(cell - 1)" class="pip" />
            </span>
          </div>
          <div
            v-for="edge in edges" :key="edge.key"
            class="strip" :class="edge.shade" :style="{ ...stripBox, transform: edge.transform }"
          />
          <div v-for="wire in wires" :key="wire.key" class="wire" :style="wire.style" />
          <div
            v-for="corner in corners" :key="corner.key"
            class="corner" :style="{ ...ballBox, transform: corner.transform }"
          >
            <div class="ball" />
          </div>
        </div>
      </div>
    </div>
  </div>
</template>

<style scoped>
.backdrop {
  position: fixed; inset: 0; z-index: -1;
  overflow: hidden; pointer-events: none;
}

/* Centre of the left margin (the content column is 920px), low on the page. */
.origin {
  position: absolute;
  left: calc((100vw - 920px) / 4);
  bottom: 18vh;
  width: 0; height: 0;
}

.rings {
  position: absolute; left: -500px; top: -500px;
  width: 1000px; height: 1000px; overflow: visible;
}
.ring {
  fill: none; stroke: #785964; stroke-width: 1.5;
  transform-box: fill-box; transform-origin: center;
  animation: spread 12s linear infinite;
}
@keyframes spread {
  from { transform: scale(0.07); opacity: 0.55; }
  to   { transform: scale(1);    opacity: 0; }
}

.stage { position: absolute; perspective: 400px; }
.die {
  position: relative; width: 100%; height: 100%;
  transform-style: preserve-3d;
  animation: tumble 18s linear infinite;
}
/* The corner discs run the exact inverse — the same three rotations, negated,
   in reverse order — so they stay square to the viewer through the tumble. */
@keyframes tumble {
  from { transform: rotateX(-20deg) rotateY(0deg)   rotateZ(0deg); }
  to   { transform: rotateX(340deg) rotateY(720deg) rotateZ(360deg); }
}
@keyframes face-viewer {
  from { transform: rotateZ(0deg)    rotateY(0deg)    rotateX(20deg); }
  to   { transform: rotateZ(-360deg) rotateY(-720deg) rotateX(-340deg); }
}

.face, .strip, .corner { position: absolute; }
.face, .strip { backface-visibility: hidden; }

.face {
  display: grid; grid-template: repeat(3, 1fr) / repeat(3, 1fr);
  padding: 3px;
  background: #fbfcfd;
}
.strip.side { background: #dce2e9; }
.strip.mid  { background: #c9d1da; }
.wire { position: absolute; background: var(--color-ink); }

.corner { transform-style: preserve-3d; }
.ball {
  width: 100%; height: 100%; border-radius: 50%;
  background: radial-gradient(circle at 40% 38%, #e6ebf0 0%, #d3dae2 55%, #c2cbd5 100%);
  animation: face-viewer 18s linear infinite;
}

.cell { display: grid; place-items: center; }
/* Drilled, not printed: shadowed on the upper inside edge. */
.pip {
  width: 8.5px; height: 8.5px; border-radius: 50%;
  background: #785964; /* the logo's beacon colour */
  box-shadow: inset 0 1.5px 1.5px rgba(48, 30, 38, 0.6), 0 0.5px 0 rgba(255, 255, 255, 0.6);
}

/* Needs ~200px of margin; below that it would sit under the content. */
@media (max-width: 1320px) {
  .backdrop { display: none; }
}

@media (prefers-reduced-motion: reduce) {
  .die  { animation: none; transform: rotateX(-24deg) rotateY(36deg); }
  .ball { animation: none; transform: rotateY(-36deg) rotateX(24deg); }
  .ring { animation: none; opacity: 0.25; }
  .ring:nth-child(1) { transform: scale(0.12); }
  .ring:nth-child(2) { transform: scale(0.24); }
  .ring:nth-child(3) { transform: scale(0.38); }
  .ring:nth-child(4) { transform: scale(0.54); }
}
</style>
