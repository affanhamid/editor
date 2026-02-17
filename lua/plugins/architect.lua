return {
  {
    "MunifTanjim/nui.nvim",
    lazy = true,
  },
  {
    "architect-nvim",
    dir = vim.fn.stdpath("config") .. "/lua/architect",
    dependencies = { "MunifTanjim/nui.nvim" },
    lazy = false,
    config = function()
      require("architect").setup({
        bridge_binary = vim.fn.expand("~/projects/editor/architect-bridge/architect-bridge"),
        socket_path = "/tmp/architect-bridge.sock",
        db_url = "postgres://architect:architect_local@localhost:5432/architect?sslmode=disable",
      })
    end,
  },
}
