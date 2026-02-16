return {
  -- Claude Code integration
  {
    "greggh/claude-code.nvim",
    dependencies = {
      "nvim-lua/plenary.nvim",
    },
    keys = {
      { "<leader>cc", "<cmd>ClaudeCode<cr>", desc = "Claude Code toggle" },
    },
    opts = {},
  },
}
