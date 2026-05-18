import { describe, expectTypeOf, it } from 'vitest';
import { pasteIntake, uploadIntake } from './client';
import type { BrowserArticle } from './types';

describe('api client browser article contracts', () => {
  it('types uploadIntake as returning a browser article', () => {
    expectTypeOf(uploadIntake).returns.toEqualTypeOf<Promise<BrowserArticle>>();
  });

  it('types pasteIntake as returning a browser article', () => {
    expectTypeOf(pasteIntake).returns.toEqualTypeOf<Promise<BrowserArticle>>();
  });
});
