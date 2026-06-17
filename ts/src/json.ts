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

import { Tabnas, TabnasError, type Plugin } from '@tabnas/parser'

// Current package version.
export const Version = '1.0.0'

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
export function registerJsonGrammar(tn: Tabnas): void {
  tn.grammar({
    // The schema version of the native-value builtins this grammar binds
    // to (object/array/reset/key/setval/push/value).
    v: 2,

    // The value tree is built ENTIRELY by the engine's native-value
    // `$`-builtins — referenced by name on the alts below; the engine
    // merges them in at load. There are NO grammar-local closures: the
    // builders are info-aware, so when info.map/list/text is enabled they
    // attach the introspection marker themselves (the json plugin used to
    // hand-write that as @jsonMapMark/@jsonListMark/@jsonText/@jsonSetval).
    // Strict JSON containers are always explicit, so @object$/@array$ take
    // the default implicit:false (no `k` config needed).
    //
    // The builtin actions used below, one line each:
    //   @reset$  — clear the parent-seeded node (so a value doesn't inherit
    //              the parent container).
    //   @object$ — allocate an empty object into the node (a MapRef under
    //              info.map).
    //   @array$  — allocate an empty array into the node (a ListRef under
    //              info.list).
    //   @key$    — capture the matched key token into a scratch slot for the
    //              pending @setval$.
    //   @setval$ — assign the just-built child value into the object under
    //              the captured key.
    //   @push$   — append the just-built child value to the array.
    //   @value$  — resolve the rule's value: a built child wins, else the
    //              scalar token (boxed with quote info under info.text).
    rule: {
      val: {
        // Opening token alternates. @reset$ clears the parent-seeded node
        // so a scalar doesn't inherit the parent container.
        open: [
          // A map: `{ ...`
          { s: '#OB', p: 'map', b: 1, a: '@reset$', g: 'map,json' },

          // A list: `[ ...`
          { s: '#OS', p: 'list', b: 1, a: '@reset$', g: 'list,json' },

          // A plain value: `"x"` `1` `true` ....
          { s: '#VAL', a: '@reset$', g: 'val,json' },
        ],

        // Closing alternates. @value$ coalesces (child wins, else the
        // scalar token) and, under info.text, boxes a string with its quote.
        close: [
          // End of input.
          { s: '#ZZ', a: '@value$', g: 'end,json' },

          // There's more JSON.
          { b: 1, a: '@value$', g: 'more,json' },
        ],
      },

      map: {
        open: [
          // An empty map: {}.
          { s: '#OB #CB', b: 1, n: { pk: 0 }, a: '@object$', g: 'map,json' },

          // Start matching map key-value pairs.
          // Reset counter n.pk as new map (for extensions).
          { s: '#OB', p: 'pair', n: { pk: 0 }, a: '@object$', g: 'map,json,pair' },
        ],
        close: [
          // End of map.
          { s: '#CB', g: 'end,json' },
        ],
      },

      list: {
        open: [
          // An empty list: [].
          { s: '#OS #CS', b: 1, a: '@array$', g: 'list,json' },

          // Start matching list elements.
          { s: '#OS', p: 'elem', a: '@array$', g: 'list,elem,json' },
        ],
        close: [
          // End of list.
          { s: '#CS', g: 'end,json' },
        ],
      },

      // sets key:val on node
      pair: {
        open: [
          // Match key-colon start of pair. @key$ captures the key.
          {
            s: '#KEY #CL',
            p: 'val',
            u: { pair: true },
            a: '@key$',
            g: 'map,pair,key,json',
          },
        ],
        close: [
          // Comma means a new pair at same pair-key level. @setval$
          // assigns the pair; on every close alt (fan-out).
          { s: '#CA', r: 'pair', a: '@setval$', g: 'map,pair,json' },

          // End of map.
          { s: '#CB', b: 1, a: '@setval$', g: 'map,pair,json' },
        ],
      },

      // push onto node
      elem: {
        open: [
          // List elements are values.
          { p: 'val', g: 'list,elem,val,json' },
        ],
        close: [
          // Next element. @push$ appends the element.
          { s: '#CA', r: 'elem', a: '@push$', g: 'list,elem,json' },

          // End of list.
          { s: '#CS', b: 1, a: '@push$', g: 'list,elem,json' },
        ],
      },
    },
  })
}

// The standard plugin form: apply the strict JSON options, then register
// the JSON grammar. `use` this on a bare engine, or pass it to `make`.
export const json: Plugin = function json(tn: Tabnas, _options?: any) {
  tn.options(JSON_OPTIONS)
  registerJsonGrammar(tn)
}

// Create a standard-JSON parser instance: a tabnas engine with the json
// plugin installed. Extra options (e.g. info.map/list/text) are applied
// after the grammar exists, mirroring the Go `Make`.
export function make(opts?: Record<string, any>): Tabnas {
  const tn = new Tabnas({ plugins: [json] })
  if (opts) {
    tn.options(opts)
  }
  return tn
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
