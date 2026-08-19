/* Copyright (c) 2026 tabnas, MIT License */

/* enginepin.test.js — the two runtimes test against ONE set of tabnas deps.
 *
 * json ships the same grammar twice and proves the two agree by running both
 * runtimes over the SAME fixtures. That proof is only worth what its inputs
 * are worth: the committed lockfile resolved @tabnas/parser 0.2.0 while
 * go/go.mod required parser/go v0.8.10, so the TS half of every shared
 * fixture ran against an engine six minor versions behind the Go half, and
 * the agreement that resulted said nothing about either.
 *
 * The lockfile was also INVALID, not merely stale. It had no entry for
 * @tabnas/support even though package.json declares it, so `npm ci` failed
 * outright ("Missing: @tabnas/support@0.3.1 from lock file"), and it resolved
 * @tabnas/debug to `link: ../../debug/ts` — a path outside the repo, which
 * installs only on the machine the lockfile was written on.
 *
 * Nothing could report any of that. The devDependency ranges are all `*`, so
 * there is no version in package.json for Renovate to bump and `npm i` keeps
 * whatever the lockfile already resolved. CI did not see it either: the
 * shared polyglot-ci workflow clones sibling repos and builds them from
 * SOURCE (`deps: "parser support debug"`), so the committed pins are unused
 * there. The skew hits every local `make test` — which is where the parity
 * claim actually gets checked.
 *
 * WHAT IS LOADED, NOT ONLY WHAT IS DECLARED. A pin is a claim; node_modules
 * is the fact. `npm update --package-lock-only` by design does not install,
 * so the files can agree about a version this run never loaded. The two
 * install modes are told apart rather than assumed:
 *
 *   - Registry install (a real directory): its version must equal the
 *     lockfile's.
 *   - Sibling source (a symlink): link_siblings in polyglot-ci REPLACES the
 *     published copy with a link to ../<dep>/ts, so the lockfile is not what
 *     the TS side loads and its version legitimately runs ahead of the last
 *     publish. Not compared — but the lock-vs-go.mod check still applies,
 *     because that is what every local run uses.
 *
 * Fixing a failure: `npm update --package-lock-only <pkg>` in ts/, then set
 * the same version in go/go.mod, `go mod tidy`, and `npm i`.
 */
'use strict'

const { describe, it } = require('node:test')
const assert = require('node:assert')
const fs = require('node:fs')
const path = require('node:path')

const REPO = path.join(__dirname, '..', '..')

// Only the dependencies BOTH runtimes name. go/debugtest is deliberately
// excluded: it is an isolated module wired up with local `replace`
// directives, so its versions are placeholders by design.
const SHARED = [
  { npm: '@tabnas/parser', mod: 'github.com/tabnas/parser/go' },
  { npm: '@tabnas/support', mod: 'github.com/tabnas/support/go' },
]


// --- the guard ------------------------------------------------------
//
// An ordinary function over DATA — no disk, no assertions — returning the
// problems it found, so every branch can be asserted to fail when it should.
// A guard whose only expression is an it() that currently passes cannot be
// told apart from one that never fires.
//
// `installed` maps package name -> {version, linked} | null.
function enginePinProblems({ lock, goMod, pkg, installed }) {
  const problems = []
  const packages = (lock && lock.packages) || {}

  for (const { npm, mod } of SHARED) {
    const entry = packages['node_modules/' + npm]
    if (null == entry) {
      problems.push(`ts/package-lock.json has no ${npm} entry`)
      continue
    }

    const m = new RegExp('^\\s*(?:require\\s+)?' +
      mod.replace(/[.\\/]/g, '\\$&') + '\\s+v(\\S+)', 'm').exec(String(goMod))
    if (null == m) {
      problems.push(`go/go.mod does not require ${mod}`)
    } else if (!entry.link && m[1] !== entry.version) {
      problems.push(
        `ts/package-lock.json pins ${npm} ${entry.version} but go/go.mod ` +
        `requires ${mod} v${m[1]}. The shared fixtures are the check that ` +
        'the two runtimes agree; run across two dependency versions they ' +
        'compare nothing.')
    }

    const got = installed && installed[npm]
    if (null != got && !got.linked && !entry.link && got.version !== entry.version) {
      problems.push(
        `node_modules/${npm} is ${got.version} but ts/package-lock.json ` +
        `pins ${entry.version}. The pins can agree with each other and still ` +
        'be a version this test run never loaded — run `npm i` in ts/.')
    }
  }

  // The @tabnas/debug entry was `link: true, resolved: ../../debug/ts`. That
  // is what `npm i` writes when a sibling checkout happens to be present on
  // the author's disk, and committing it makes the lockfile unreproducible
  // everywhere else. Not specific to SHARED, so checked across the file.
  for (const [name, e] of Object.entries(packages)) {
    if ('' !== name && e && e.link) {
      problems.push(
        `ts/package-lock.json resolves ${name} to a local link ` +
        `(${e.resolved}) — a committed lockfile that points outside the repo ` +
        'installs only on the machine it was written on')
    }
  }

  // package.json and package-lock.json disagreeing is not a skew, it is a
  // broken lockfile: `npm ci` refuses to install at all. Read package.json
  // ITSELF, never the lockfile's copy of it — the broken lockfile this was
  // written for was missing @tabnas/support from both, so a check reading
  // the lockfile's own idea of package.json agreed with itself and passed.
  const declared = Object.keys((pkg && pkg.devDependencies) || {})
    .concat(Object.keys((pkg && pkg.dependencies) || {}))
  const missing = declared.filter((d) => null == packages['node_modules/' + d])
  if (0 < missing.length) {
    problems.push(
      `declared in ts/package.json but absent from ts/package-lock.json, so ` +
      `\`npm ci\` fails outright: ${missing.join(', ')}. Run \`npm i\` in ts/.`)
  }

  return problems
}


// --- reading the real repository ------------------------------------

function readInstalled(name) {
  const dir = path.join(REPO, 'ts', 'node_modules', name)
  let st
  try {
    // lstat, not stat: the symlink IS the signal that this came from a
    // sibling checkout rather than the registry.
    st = fs.lstatSync(dir)
  } catch {
    return null
  }
  let version = null
  try {
    version = JSON.parse(
      fs.readFileSync(path.join(dir, 'package.json'), 'utf8')).version
  } catch { /* present but unreadable; reported as a mismatch below */ }
  return { version, linked: st.isSymbolicLink() }
}

const read = (...p) => fs.readFileSync(path.join(REPO, ...p), 'utf8')
const REAL = {
  lock: JSON.parse(read('ts', 'package-lock.json')),
  goMod: read('go', 'go.mod'),
  pkg: JSON.parse(read('ts', 'package.json')),
  installed: Object.fromEntries(
    SHARED.map(({ npm }) => [npm, readInstalled(npm)])),
}


// --- fixtures -------------------------------------------------------

const PKG = { devDependencies: { '@tabnas/parser': '*', '@tabnas/support': '*' } }
const lockOf = (parser, support, extra) => ({
  packages: {
    '': {},
    'node_modules/@tabnas/parser': { version: parser },
    'node_modules/@tabnas/support': { version: support },
    ...extra,
  },
})
const goModOf = (parser, support) =>
  `require github.com/tabnas/parser/go v${parser}\n` +
  `require github.com/tabnas/support/go v${support}\n`
const base = (over) => ({
  lock: lockOf('0.8.10', '0.3.2'),
  goMod: goModOf('0.8.10', '0.3.2'),
  pkg: PKG,
  installed: null,
  ...over,
})


describe('engine pin', () => {

  it('this repository pins one version of each shared dependency', () => {
    const mode = SHARED.map(({ npm }) => {
      const g = REAL.installed[npm]
      return `${npm}: ` + (null == g ? 'not installed'
        : g.linked ? `sibling source (${g.version})`
          : `registry install (${g.version})`)
    }).join(', ')
    assert.deepEqual(
      enginePinProblems(REAL), [], `engine pin problems (${mode})`)
  })

  describe('fails when it should', () => {

    it('lockfile and go.mod name different versions', () => {
      const p = enginePinProblems(base({ goMod: goModOf('0.2.0', '0.3.2') }))
      assert.equal(p.length, 1)
      assert.match(p[0], /pins @tabnas\/parser 0\.8\.10 but .*v0\.2\.0/)
    })

    it('the lockfile has no entry for a shared dependency', () => {
      const lock = lockOf('0.8.10', '0.3.2')
      delete lock.packages['node_modules/@tabnas/support']
      const p = enginePinProblems(base({ lock }))
      assert.deepEqual(p, [
        'ts/package-lock.json has no @tabnas/support entry',
        'declared in ts/package.json but absent from ts/package-lock.json, ' +
        'so `npm ci` fails outright: @tabnas/support. Run `npm i` in ts/.',
      ])
    })

    it('a lockfile entry is a local link', () => {
      const p = enginePinProblems(base({
        lock: lockOf('0.8.10', '0.3.2', {
          'node_modules/@tabnas/debug': { link: true, resolved: '../../debug/ts' },
        }),
      }))
      assert.equal(p.length, 1)
      assert.match(p[0], /@tabnas\/debug to a local link \(\.\.\/\.\.\/debug\/ts\)/)
    })

    it('go.mod does not require a shared dependency', () => {
      const p = enginePinProblems(base({ goMod: 'module x\n' }))
      assert.deepEqual(p, [
        'go/go.mod does not require github.com/tabnas/parser/go',
        'go/go.mod does not require github.com/tabnas/support/go',
      ])
    })

    it('node_modules is stale against a lockfile-only update', () => {
      const p = enginePinProblems(base({
        installed: { '@tabnas/parser': { version: '0.2.0', linked: false } },
      }))
      assert.equal(p.length, 1)
      assert.match(p[0], /node_modules\/@tabnas\/parser is 0\.2\.0 but/)
    })
  })

  // A guard that fires when it SHOULDN'T is just as broken, and this is the
  // case CI runs on every push.
  it('a sibling-source symlink ahead of the lockfile is not a problem', () => {
    assert.deepEqual(
      enginePinProblems(base({
        installed: {
          '@tabnas/parser': { version: '0.9.0-dev', linked: true },
          '@tabnas/support': { version: '0.4.0-dev', linked: true },
        },
      })),
      [],
    )
  })
})
