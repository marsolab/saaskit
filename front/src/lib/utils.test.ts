import { describe, expect, test } from 'bun:test';

import { cn } from './utils';

describe('cn', () => {
  test('joins class names', () => {
    expect(cn('a', 'b')).toBe('a b');
  });

  test('drops falsy values', () => {
    expect(cn('a', false, undefined, null, 'b')).toBe('a b');
  });

  test('merges conflicting tailwind classes, last wins', () => {
    expect(cn('px-2', 'px-4')).toBe('px-4');
  });
});
