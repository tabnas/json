/* Copyright (c) 2026 tabnas, MIT License */

/* json-cli.ts
 *
 * A tiny command-line front end: read JSON from the arguments or from
 * stdin, parse it, and print the canonical re-serialized form. Exit code
 * 1 (with the error on stderr) when the input is not valid JSON.
 *
 * The logic is split into `run` (pure: source in, exit code out, writes
 * via injected sinks) and `main` (process wiring), so `run` and `main`
 * are both testable in-process. `bin/json` calls `main`.
 */

import { parse, TabnasError } from './json'

// Parse `src`, writing the pretty-printed value to `out` on success or
// the error message to `errOut` on failure. Returns the process exit code.
export function run(
  src: string,
  out: (s: string) => void,
  errOut: (s: string) => void,
): number {
  try {
    out(JSON.stringify(parse(src), null, 2) + '\n')
    return 0
  } catch (err) {
    if (err instanceof TabnasError) {
      errOut(err.message + '\n')
      return 1
    }
    throw err
  }
}

// Wire `run` to a process-like environment. With arguments, parse them;
// otherwise read all of stdin. Calls `exit` with the resulting code.
export function main(
  argv: string[],
  stdin: NodeJS.ReadableStream,
  out: (s: string) => void,
  errOut: (s: string) => void,
  exit: (code: number) => void,
): void {
  if (argv.length > 0) {
    exit(run(argv.join(' '), out, errOut))
    return
  }
  const chunks: Buffer[] = []
  stdin.on('data', (c: Buffer) => chunks.push(Buffer.from(c)))
  stdin.on('end', () => exit(run(Buffer.concat(chunks).toString('utf8'), out, errOut)))
}
