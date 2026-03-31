#!/usr/bin/env node
import { Command } from 'commander';
import { mkdir, writeFile } from 'node:fs/promises';
import { dirname } from 'node:path';

import { loadJsonFile } from '../core/io.js';
import { renderArticle } from '../core/render.js';
import { validateArticle } from '../core/validate.js';
import { runPipeline } from '../core/pipeline.js';

const program = new Command();
program.name('wechat-claw-js');

program
  .command('render')
  .argument('<input>')
  .option('-o, --output <path>')
  .option('--check')
  .action(async (input, options) => {
    try {
      const article = await loadJsonFile(input);
      const html = await renderArticle(article);

      if (options.check) {
        const result = await validateArticle(article, { htmlText: html });
        if (!result.ok) {
          console.error(JSON.stringify(result, null, 2));
          process.exitCode = 1;
          return;
        }
      }

      if (options.output) {
        await mkdir(dirname(options.output), { recursive: true });
        await writeFile(options.output, html, 'utf8');
        console.log(options.output);
      } else {
        console.log(html);
      }
    } catch (error) {
      console.error(error.message);
      process.exitCode = 1;
    }
  });

program
  .command('validate')
  .argument('<input>')
  .option('--html <path>')
  .action(async (input, options) => {
    try {
      const article = await loadJsonFile(input);
      const htmlText = options.html ? await (await import('node:fs/promises')).readFile(options.html, 'utf8') : null;
      const result = await validateArticle(article, { htmlText });
      console.log(JSON.stringify(result, null, 2));
      if (!result.ok) {
        process.exitCode = 1;
      }
    } catch (error) {
      console.error(error.message);
      process.exitCode = 1;
    }
  });

program
  .command('pipeline')
  .argument('<input>')
  .option('--output-dir <path>', 'output directory', 'build')
  .option('--dry-run')
  .action(async (input, options) => {
    try {
      const summary = await runPipeline({
        input,
        outputDir: options.outputDir,
        dryRun: Boolean(options.dryRun)
      });
      console.log(JSON.stringify(summary, null, 2));
    } catch (error) {
      console.error(error.message);
      process.exitCode = 1;
    }
  });

await program.parseAsync(process.argv);
