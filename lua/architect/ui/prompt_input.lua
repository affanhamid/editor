local NuiInput = require("nui.input")

local M = {}

function M.show(bridge)
  local input = NuiInput({
    position = "50%",
    size = { width = 80 },
    border = {
      style = "rounded",
      text = { top = " Architect Prompt ", top_align = "center" },
    },
  }, {
    prompt = "  > ",
    on_submit = function(value)
      if value and value ~= "" then
        bridge.send({
          type = "submit_prompt",
          data = { prompt = value },
        })
        vim.notify("Prompt submitted to orchestrator", vim.log.levels.INFO)
      end
    end,
  })
  input:mount()
  input:map("n", "<Esc>", function() input:unmount() end)
end

return M
