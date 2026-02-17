local NuiPopup = require("nui.popup")
local utils = require("architect.utils")

local M = {}

function M.show(tasks, edges)
  local popup = NuiPopup({
    position = "50%",
    size = { width = 80, height = 30 },
    border = { style = "rounded", text = { top = " Task DAG " } },
    buf_options = { modifiable = false, filetype = "architect-dag" },
  })
  popup:mount()

  local lines = {}
  for _, task in ipairs(tasks) do
    local icon = utils.task_icons[task.status] or "?"
    local assignee = task.assigned_to and (" [" .. utils.short_id(task.assigned_to) .. "]") or ""
    local deps = {}
    for _, edge in ipairs(edges) do
      if edge.to_task == task.id then
        table.insert(deps, "#" .. edge.from_task)
      end
    end
    local dep_str = #deps > 0 and (" ← " .. table.concat(deps, ", ")) or ""
    table.insert(lines, string.format("  %s #%d: %s%s%s", icon, task.id, task.title, assignee, dep_str))
  end

  vim.bo[popup.bufnr].modifiable = true
  vim.api.nvim_buf_set_lines(popup.bufnr, 0, -1, false, lines)
  vim.bo[popup.bufnr].modifiable = false

  popup:map("n", "q", function() popup:unmount() end)
  popup:map("n", "<Esc>", function() popup:unmount() end)

  M.popup = popup
end

return M
