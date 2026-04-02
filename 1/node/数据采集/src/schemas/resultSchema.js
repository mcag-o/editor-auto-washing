import { z } from 'zod';

export const itemSchema = z.object({
  id: z.string(),
  rank: z.number().int().positive(),
  title: z.string(),
  url: z.string().url(),
  mobileUrl: z.string().url().nullable(),
  hot: z.string().nullable(),
  summary: z.string().nullable(),
  author: z.string().nullable(),
  category: z.string().nullable(),
  tags: z.array(z.string()),
  publishTime: z.string().nullable(),
  metadata: z.record(z.string(), z.any()),
  raw: z.any()
});

export const errorSchema = z.object({
  code: z.string(),
  message: z.string(),
  retryable: z.boolean()
});

export const resultSchema = z.object({
  platform: z.string(),
  canonicalPlatform: z.string(),
  aliases: z.array(z.string()),
  displayName: z.string(),
  sourceType: z.enum(['json-api', 'html', 'browser']),
  sourceUrl: z.string().url(),
  fetchedAt: z.string(),
  success: z.boolean(),
  items: z.array(itemSchema),
  warnings: z.array(z.string()),
  error: errorSchema.optional()
});
