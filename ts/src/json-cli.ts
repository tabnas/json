/* Copyright (c) 2026 tabnas, MIT License */

/* json-cli.ts
 *
 * A tiny command-line front end: read JSON from the arguments or from
 * stdin, parse it, and print the canonical re-serialized form. Exit code
 * 1 (with the error on stderr) when the input is not valid JSON.
 */

import { parse, JsonError } from './json'

function run(src: string): number {
  try {
    const value = parse(src)
    process.stdout.write(JSON.stringify(value, null, 2) + '\n')
    return 0
  } catch (err) {
    if (err instanceof JsonError) {
      process.stderr.write(err.message + '\n')
      return 1
    }
    throw err
  }
}

function main(): void {
  const args = process.argv.slice(2)
  if (args.length > 0) {
    process.exit(run(args.join(' ')))
  }
  const chunks: Buffer[] = []
  process.stdin.on('data', (c: Buffer) => chunks.push(c))
  process.stdin.on('end', () => {
    process.exit(run(Buffer.concat(chunks).toString('utf8')))
  })
}

main()
