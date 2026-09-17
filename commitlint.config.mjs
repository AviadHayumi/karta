// SPDX-License-Identifier: Apache-2.0
// Copyright (c) 2026 NVIDIA Corporation

export default {
  // Merge, revert, and fixup commits must also satisfy the attribution policy.
  defaultIgnores: false,
  plugins: [{
    rules: {
      'no-ai-coauthored-by': ({ raw }) => [
        !/^[ \t]*Co-authored-by[ \t]*:.*\b(Claude|ChatGPT|Copilot|Codex|Devin|Cursor|Gemini|Anthropic)\b/im.test(raw),
        'Remove AI Co-authored-by trailers; keep human co-authors and Signed-off-by trailers.',
      ],
    },
  }],
  rules: {
    'no-ai-coauthored-by': [2, 'always'],
  },
};
