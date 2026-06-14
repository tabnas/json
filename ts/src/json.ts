/* Copyright (c) 2026 tabnas, MIT License */

/*  json.ts
 *
 *  A standard JSON grammar plugin for the `tabnas` parsing engine.
 *
 *  The engine ships no grammar of its own; this package supplies the
 *  strict, standard-JSON one. The rule set (val / map / list / pair /
 *  elem) is the "Plain JSON" grammar from jsonic's `grammar.ts` — the
 *  pure-JSON core jsonic defines before extending it for the relaxed
 *  jsonic format. Here that core is installed on its own, with the lexer
 *  restricted to strict JSON, and none of jsonic's extended grammar
 *  (comments, unquoted keys, implicit objects/arrays, trailing commas,
 *  single/backtick strings, path diving).
 *
 *  This plugin is intended to be the foundation other tabnas grammar
 *  plugins build on: `use` it first, then layer additional rules on the
 *  shared val / map / list / pair / elem rules.
 */

import {
  Tabnas,
  TabnasError,
  type Context,
  type FuncRef,
  type Plugin,
  type Rule,
} from 'tabnas'

// Current package version.
export const Version = '1.0.0'

const defprop = Object.defineProperty

// Attach a hidden marker property to a node — used when info.map /
// info.list mode is on so callers can introspect container origins.
// Standard JSON parsing ignores marker properties.
function mark(node: any, marker: string, data: any): void {
  if (node != null && typeof node === 'object') {
    defprop(node, marker, { value: data, writable: true })
  }
}

// JSON-only lexer/parser options. Restrictive enough to mirror
// JSON.parse: double-quoted strings only, plain decimal numbers, quoted
// keys only, no comments, no trailing-comma map extension, no empty
// input, and the rule set restricted to the `json`-tagged alternates.
const JSON_OPTIONS = {
  text: { lex: false },
  number: {
    hex: false,
    oct: false,
    bin: false,
    sep: null,
    // Reject any token the (lenient) number matcher accepts that is not a
    // strict JSON number: leading `+`, a bare leading `.` (`.5`), a
    // trailing `.` (`1.`), and leading zeros (`01`, `00`). The negative
    // lookahead matches — and so excludes — anything not of the form
    // -?(0|[1-9][0-9]*)(.[0-9]+)?([eE][+-]?[0-9]+)?.
    exclude: /^(?!-?(?:0|[1-9][0-9]*)(?:\.[0-9]+)?(?:[eE][+-]?[0-9]+)?$)/,
  },
  string: {
    chars: '"',
    multiChars: '',
    // Standard JSON escape handling: allowUnknown:false rejects any
    // unrecognized escape (\q, \z); escapeStrict disables the engine's
    // non-standard \xHH and \u{...} structural escapes (plain \uXXXX
    // stays); and dropping v / ' / ` from the escape map removes the
    // remaining non-standard built-ins. Result: exactly the JSON.parse
    // escape set, identical to the Go engine.
    allowUnknown: false,
    escapeStrict: true,
    escape: { v: '', "'": '', '`': '' },
  },
  comment: { lex: false },
  map: { extend: false },
  lex: { empty: false },
  rule: { finish: false, include: 'json' },
  result: { fail: [undefined, NaN] },
  tokenSet: { KEY: ['#ST', null, null, null] },
}

// Install the pure JSON rule set (val / map / list / pair / elem) on the
// given engine instance. Exposed separately from the options so other
// grammar plugins can layer their extensions on top without re-declaring
// the JSON core. This is jsonic's "Plain JSON" grammar.
export function registerJsonGrammar(am: Tabnas): void {
  am.grammar({
    ref: {
      // Strict JSON keys are quoted strings (the KEY token set is
      // restricted to #ST), so the key value is the decoded string.
      '@pairkey': (r: Rule) => {
        r.u.key = r.o0.val
      },

      '@val-bo': (rule: Rule) => (rule.node = undefined),
      '@val-bc': (r: Rule, ctx: Context) => {
        // A map/list child node wins; otherwise the value is the scalar
        // token. (The strict lexer guarantees a value rule always has one
        // or the other — there are no empty values to coalesce.)
        if (undefined !== r.child.node) {
          r.node = r.child.node
          return
        }
        let val = r.o0.resolveVal(r, ctx)
        if (
          ctx.cfg.info.text &&
          typeof val === 'string' &&
          (r.o0.tin === ctx.cfg.t.ST || r.o0.tin === ctx.cfg.t.TX)
        ) {
          const quote =
            r.o0.tin === ctx.cfg.t.ST && r.o0.src.length > 0 ? r.o0.src[0] : ''
          const sv = new String(val)
          mark(sv, ctx.cfg.info.marker, { quote })
          val = sv as any
        }
        r.node = val
      },

      '@map-bo': (r: Rule, ctx: Context) => {
        // Create a new empty map.
        r.node = Object.create(null)
        if (ctx.cfg.info.map) {
          mark(r.node, ctx.cfg.info.marker, { implicit: false, meta: {} })
        }
      },

      '@list-bo': (r: Rule, ctx: Context) => {
        // Create a new empty list.
        r.node = []
        if (ctx.cfg.info.list) {
          mark(r.node, ctx.cfg.info.marker, { implicit: false, meta: {} })
        }
      },

      '@pair-bc': (r: Rule, ctx: Context) => {
        if (r.u.pair) {
          // Drop keys that match the info marker to preserve metadata.
          if (ctx.cfg.info.map && r.u.key === ctx.cfg.info.marker) {
            return
          }
          // Store previous value (if any, for extensions).
          r.u.prev = r.node[r.u.key]
          r.node[r.u.key] = r.child.node
        }
      },

      '@elem-bc': (r: Rule) => {
        if (true !== r.u.done && undefined !== r.child.node) {
          r.node.push(r.child.node)
        }
      },
    } as Record<FuncRef, Function>,

    rule: {
      val: {
        // Opening token alternates.
        open: [
          // A map: `{ ...`
          { s: '#OB', p: 'map', b: 1, g: 'map,json' },

          // A list: `[ ...`
          { s: '#OS', p: 'list', b: 1, g: 'list,json' },

          // A plain value: `"x"` `1` `true` ....
          { s: '#VAL', g: 'val,json' },
        ],

        // Closing token alternates.
        close: [
          // End of input.
          { s: '#ZZ', g: 'end,json' },

          // There's more JSON.
          { b: 1, g: 'more,json' },
        ],
      },

      map: {
        open: [
          // An empty map: {}.
          { s: '#OB #CB', b: 1, n: { pk: 0 }, g: 'map,json' },

          // Start matching map key-value pairs.
          // Reset counter n.pk as new map (for extensions).
          { s: '#OB', p: 'pair', n: { pk: 0 }, g: 'map,json,pair' },
        ],
        close: [
          // End of map.
          { s: '#CB', g: 'end,json' },
        ],
      },

      list: {
        open: [
          // An empty list: [].
          { s: '#OS #CS', b: 1, g: 'list,json' },

          // Start matching list elements.
          { s: '#OS', p: 'elem', g: 'list,elem,json' },
        ],
        close: [
          // End of list.
          { s: '#CS', g: 'end,json' },
        ],
      },

      // sets key:val on node
      pair: {
        open: [
          // Match key-colon start of pair.
          {
            s: '#KEY #CL',
            p: 'val',
            u: { pair: true },
            a: '@pairkey',
            g: 'map,pair,key,json',
          },
        ],
        close: [
          // Comma means a new pair at same pair-key level.
          { s: '#CA', r: 'pair', g: 'map,pair,json' },

          // End of map.
          { s: '#CB', b: 1, g: 'map,pair,json' },
        ],
      },

      // push onto node
      elem: {
        open: [
          // List elements are values.
          { p: 'val', g: 'list,elem,val,json' },
        ],
        close: [
          // Next element.
          { s: '#CA', r: 'elem', g: 'list,elem,json' },

          // End of list.
          { s: '#CS', b: 1, g: 'list,elem,json' },
        ],
      },
    },
  })
}

// The standard plugin form: apply the strict JSON options, then register
// the JSON grammar. `use` this on a bare engine, or pass it to `make`.
export const json: Plugin = function json(am: Tabnas, _options?: any) {
  am.options(JSON_OPTIONS)
  registerJsonGrammar(am)
}

// Create a standard-JSON parser instance: a tabnas engine with the json
// plugin installed. Extra options (e.g. info.map/list/text) are applied
// after the grammar exists, mirroring the Go `Make`.
export function make(opts?: Record<string, any>): Tabnas {
  const am = new Tabnas({ plugins: [json] })
  if (opts) {
    am.options(opts)
  }
  return am
}

// A lazily-created default instance reused by `parse`, so repeated calls
// don't rebuild the engine and grammar each time. Parsing creates a fresh
// context per call, so reuse is safe.
let defaultParser: Tabnas | undefined

// Parse a JSON source string with a default standard-JSON parser and
// return the resulting value. Throws a TabnasError on invalid input.
export function parse(src: string): any {
  return (defaultParser ??= make()).parse(src)
}

export { Tabnas, TabnasError }
export { TabnasError as JsonError }
export default parse
