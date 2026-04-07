import readline from 'node:readline/promises';
import { stdin as input, stdout as output } from 'node:process';
import { run } from './run.js';
import { runSourcesCli } from './sources.js';

function printMenu() {
  output.write('1) 列出全部收集途径与启用状态\n');
  output.write('2) 检查全部收集途径有效性\n');
  output.write('3) 执行一次采集（启用源，不落盘）\n');
  output.write('4) 执行一次采集并按来源写入多个 JSON 文件\n');
  output.write('0) 退出\n');
}

export async function runOpsCli() {
  const rl = readline.createInterface({ input, output });

  try {
    while (true) {
      printMenu();
      const answer = (await rl.question('请选择序号: ')).trim();

      if (answer === '0') {
        break;
      }

      if (answer === '1') {
        await runSourcesCli(['list']);
        continue;
      }

      if (answer === '2') {
        await runSourcesCli(['check']);
        continue;
      }

      if (answer === '3') {
        await run(['--all', '--no-output']);
        continue;
      }

      if (answer === '4') {
        await run(['--all']);
        continue;
      }

      output.write('无效序号，请输入 0~4。\n');
    }
  } finally {
    rl.close();
  }
}

if (import.meta.url === `file://${process.argv[1]}`) {
  await runOpsCli();
}
