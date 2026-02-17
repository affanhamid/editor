local NuiPopup = require("nui.popup")
local NuiInput = require("nui.input")

local M = {}

function M.show(consultation, bridge)
  local popup = NuiPopup({
    position = "50%",
    size = { width = 70, height = 20 },
    border = {
      style = "double",
      text = { top = " CONSULTATION REQUEST ", top_align = "center" },
    },
    buf_options = { modifiable = false },
  })
  popup:mount()

  local lines = {
    "",
    "  Agent: " .. (consultation.agent_id and consultation.agent_id:sub(1, 8) or "unknown"),
    "  Task:  #" .. (consultation.task_id or "?") .. " — " .. (consultation.task_title or ""),
    "  Risk:  " .. (consultation.risk_level or "unknown"),
    "",
    "  " .. string.rep("─", 60),
    "",
  }
  if consultation.description then
    for line in consultation.description:gmatch("[^\n]+") do
      table.insert(lines, "  " .. line)
    end
  end
  table.insert(lines, "")
  table.insert(lines, "  " .. string.rep("─", 60))
  table.insert(lines, "")
  table.insert(lines, "  [a] Approve    [r] Reject    [m] Modify    [q] Dismiss")

  vim.bo[popup.bufnr].modifiable = true
  vim.api.nvim_buf_set_lines(popup.bufnr, 0, -1, false, lines)
  vim.bo[popup.bufnr].modifiable = false

  popup:map("n", "a", function()
    bridge.send({ type = "approve_consultation", data = { task_id = consultation.task_id, approved = true } })
    popup:unmount()
    vim.notify("Consultation approved", vim.log.levels.INFO)
  end)

  popup:map("n", "r", function()
    bridge.send({ type = "approve_consultation", data = { task_id = consultation.task_id, approved = false } })
    popup:unmount()
    vim.notify("Consultation rejected", vim.log.levels.WARN)
  end)

  popup:map("n", "m", function()
    popup:unmount()
    local input = NuiInput({
      position = "50%",
      size = { width = 70 },
      border = {
        style = "rounded",
        text = { top = " Modified Instructions ", top_align = "center" },
      },
    }, {
      prompt = "  > ",
      on_submit = function(value)
        if value and value ~= "" then
          bridge.send({
            type = "approve_consultation",
            data = { task_id = consultation.task_id, approved = true, note = value },
          })
          vim.notify("Consultation approved with modifications", vim.log.levels.INFO)
        end
      end,
    })
    input:mount()
    input:map("n", "<Esc>", function() input:unmount() end)
  end)

  popup:map("n", "q", function() popup:unmount() end)
  popup:map("n", "<Esc>", function() popup:unmount() end)
end

return M
