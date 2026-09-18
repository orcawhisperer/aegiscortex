(function () {
  "use strict";

  var state = {
    presets: [],
    activeScenarioId: "rag_verified_fastpath",
    flywheel: null,
    debounceTimer: null
  };

  function el(tag, className, text) {
    var node = document.createElement(tag);
    if (className) node.className = className;
    if (text !== undefined && text !== null) node.textContent = String(text);
    return node;
  }

  function $(id) {
    return document.getElementById(id);
  }

  function setStatus(msg, isError) {
    var banner = $("status-banner");
    if (!banner) return;
    banner.textContent = msg;
    banner.className = isError ? "status error" : "status";
  }

  function updateModePill(mode, hasKey, fallback) {
    var pill = $("mode-indicator");
    var textNode = $("mode-text");
    if (!pill || !textNode) return;
    var live = hasKey || mode === "LIVE_TYPESAFE_API";
    pill.className = "mode " + (live && !fallback ? "live" : "sim");
    if (fallback) {
      textNode.textContent = "Live failed — simulator";
    } else if (live) {
      textNode.textContent = "Live API";
    } else {
      textNode.textContent = "Simulator";
    }
  }

  function renderScenarioButtons() {
    var container = $("scenario-list");
    if (!container) return;
    var nodes = [];
    for (var i = 0; i < state.presets.length; i++) {
      var p = state.presets[i];
      var btn = el("button", "scenario-btn" + (p.id === state.activeScenarioId ? " active" : ""));
      btn.type = "button";
      btn.setAttribute("data-scenario-id", p.id);
      var top = el("div", "scenario-top");
      var title = el("span", "scenario-title");
      title.appendChild(el("span", "scenario-index", String(i + 1).padStart(2, "0")));
      title.appendChild(document.createTextNode(p.title));
      top.appendChild(title);
      top.appendChild(el("span", "scenario-tag", p.badge));
      btn.appendChild(top);
      btn.appendChild(el("p", "scenario-desc", p.description));
      (function (preset) {
        btn.addEventListener("click", function () {
          state.activeScenarioId = preset.id;
          var editor = $("context-editor");
          if (editor) editor.value = JSON.stringify(preset.context, null, 2);
          renderScenarioButtons();
          evaluateCurrent();
        });
      })(p);
      nodes.push(btn);
    }
    container.replaceChildren.apply(container, nodes);
  }

  function renderTable(container, headers, rows, rowClasses) {
    var table = el("table");
    var thead = el("thead");
    var hr = el("tr");
    headers.forEach(function (h) { hr.appendChild(el("th", null, h)); });
    thead.appendChild(hr);
    table.appendChild(thead);
    var tbody = el("tbody");
    rows.forEach(function (cells, i) {
      var tr = el("tr", rowClasses && rowClasses[i] ? rowClasses[i] : "");
      cells.forEach(function (c) {
        var td = el("td");
        if (c && c.nodeType) td.appendChild(c);
        else td.textContent = c == null ? "" : String(c);
        tr.appendChild(td);
      });
      tbody.appendChild(tr);
    });
    table.appendChild(tbody);
    container.replaceChildren(table);
  }

  function renderFieldVerifications(fields) {
    var grid = $("field-verifications-grid");
    if (!grid) return;
    var rows = [];
    var cls = [];
    for (var i = 0; i < (fields || []).length; i++) {
      var f = fields[i];
      rows.push([
        f.field_name,
        (f.yes_prob * 100).toFixed(0) + "%",
        f.confidence.toFixed(2),
        f.verified ? "lock" : "repair"
      ]);
      cls.push(f.verified ? "ok" : "fail");
    }
    renderTable(grid, ["Field", "P(yes)", "Conf", "Gate"], rows, cls);
  }

  function renderQuestionsMatrix(questions) {
    var matrix = $("questions-matrix");
    if (!matrix) return;
    var rows = [];
    for (var i = 0; i < (questions || []).length; i++) {
      var q = questions[i];
      var prompt = el("div");
      prompt.appendChild(el("div", null, q.id));
      prompt.appendChild(el("div", "q-prompt", q.prompt));
      rows.push([
        prompt,
        q.stage,
        q.selected_choice,
        (q.top_probability * 100).toFixed(0) + "%",
        q.route_reason.replace("Gate: ", "")
      ]);
    }
    renderTable(matrix, ["Question", "Stage", "Answer", "P", "Gate"], rows);
  }

  function renderHistory(rows) {
    var list = $("history-list");
    if (!list) return;
    if (!rows || !rows.length) {
      list.replaceChildren(el("p", "note", "No runs yet."));
      return;
    }
    var body = [];
    for (var i = 0; i < rows.length; i++) {
      var h = rows[i];
      body.push([h.timestamp, h.final_route_tier, h.latency_ms.toFixed(0) + " ms"]);
    }
    renderTable(list, ["Time", "Route", "ms"], body);
  }

  function tierClass(tier) {
    if (tier === "TIER_0_BLOCK") return "stamp tier-block";
    if (tier === "TIER_0_AUTO_EXEC") return "stamp tier-auto";
    if (tier && tier.indexOf("TIER_2") === 0) return "stamp tier-repair";
    return "stamp tier-fastpath";
  }

  function updateDashboard(res) {
    if (!res) return;
    updateModePill(res.mode, res.mode === "LIVE_TYPESAFE_API", res.fallback_used);
    if (res.cascade) {
      $("kpi-latency").textContent = res.latency_ms.toFixed(0) + " ms";
      $("kpi-latency-sub").textContent = "Jev P50 " + res.cascade.reference_jev_p50_ms.toFixed(0) + " ms";
      $("kpi-jev-cost").textContent = "$" + res.cascade.aegis_control_cost_usd.toFixed(6);
      $("kpi-savings-pct").textContent = res.cascade.cost_savings_percent.toFixed(1) + "%";
      $("kpi-savings-sub").textContent = "baseline $" + res.cascade.naive_frontier_cost_usd.toFixed(5);
      $("arch-naive-cost").textContent = "$" + res.cascade.naive_frontier_cost_usd.toFixed(6);
      $("arch-naive-lat").textContent = res.cascade.naive_frontier_latency_ms.toFixed(0) + " ms";
      $("arch-legacy-cost").textContent = "$" + res.cascade.legacy_router_cost_usd.toFixed(6);
      $("arch-legacy-lat").textContent = res.cascade.legacy_router_latency_ms.toFixed(0) + " ms";
      $("arch-aegis-cost").textContent = "$" + res.cascade.aegis_blended_cost_usd.toFixed(6);
      $("arch-aegis-lat").textContent = res.cascade.aegis_total_latency_ms.toFixed(0) + " ms";
    }
    $("kpi-composite").textContent = res.composite_score.toFixed(1);
    $("kpi-route-tier").textContent = res.final_route_tier;
    if (res.flywheel) {
      state.flywheel = res.flywheel;
      $("kpi-flywheel").textContent = String(res.flywheel.distilled_golden_examples);
      $("kpi-flywheel-sub").textContent = res.flywheel.guardrails_blocked + " block · " + res.flywheel.auto_executed + " auto";
      $("recommended-line").textContent =
        "τ  sec=" + res.flywheel.recommended_block_prob.toFixed(2) +
        "  field=" + res.flywheel.recommended_verify_min.toFixed(2) +
        "  route=" + res.flywheel.recommended_act_gate.toFixed(2) +
        "  composite=" + res.flywheel.recommended_composite.toFixed(0) +
        "  ECE=" + res.flywheel.expected_calibration_ece.toFixed(3);
    }
    $("decision-tier-title").textContent = res.final_route_tier;
    var badge = $("decision-badge");
    badge.textContent = res.final_route_tier;
    badge.className = tierClass(res.final_route_tier);
    $("decision-rationale").textContent = res.final_decision;
    var failed = $("failed-fields-line");
    if (res.failed_fields && res.failed_fields.length) {
      failed.textContent = "Fields in repair: " + res.failed_fields.join(", ");
    } else {
      failed.textContent = res.selected_skill ? "Skill: " + res.selected_skill : "";
    }
    renderFieldVerifications(res.field_verifications || []);
    renderQuestionsMatrix(res.questions || []);
    renderHistory(res.history || []);
    if (res.live_error) {
      setStatus("Live API error, using simulator: " + res.live_error, true);
    }
  }

  function readThresholds() {
    return {
      security_gate_confidence: parseFloat($("slider-security").value),
      field_verify_confidence: parseFloat($("slider-field").value),
      router_confidence: parseFloat($("slider-router").value),
      composite_pass_threshold: parseFloat($("slider-composite").value)
    };
  }

  function paintThresholdLabels(payload) {
    $("val-security").textContent = payload.security_gate_confidence.toFixed(2);
    $("val-field").textContent = payload.field_verify_confidence.toFixed(2);
    $("val-router").textContent = payload.router_confidence.toFixed(2);
    $("val-composite").textContent = payload.composite_pass_threshold.toFixed(0);
  }

  function evaluateCurrent() {
    var editor = $("context-editor");
    var parsedContext = {};
    if (editor && editor.value.trim() !== "") {
      try {
        parsedContext = JSON.parse(editor.value);
      } catch (e) {
        setStatus("Invalid JSON: " + e.message, true);
        return;
      }
    }
    setStatus("Running 11-question fan-out…");
    fetch("/api/evaluate", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        scenario_id: state.activeScenarioId,
        context: parsedContext
      })
    })
      .then(function (r) { return r.json().then(function (data) { return { ok: r.ok, data: data }; }); })
      .then(function (res) {
        if (!res.ok) {
          setStatus(res.data.error || "Evaluation failed", true);
          return;
        }
        updateDashboard(res.data);
        setStatus(
          "Route " + res.data.final_route_tier + " in " + res.data.latency_ms.toFixed(0) + " ms wall-clock · " +
            res.data.questions.length + " questions · savings " + res.data.cascade.cost_savings_percent.toFixed(1) + "%"
        );
      })
      .catch(function (err) {
        setStatus("Evaluation failed: " + err.message, true);
      });
  }

  function syncThresholds() {
    var payload = readThresholds();
    paintThresholdLabels(payload);
    fetch("/api/thresholds", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload)
    }).then(function () {
      evaluateCurrent();
    });
  }

  function init() {
    var bootNode = $("boot-data");
    if (bootNode && bootNode.textContent) {
      try {
        var boot = JSON.parse(bootNode.textContent);
        state.presets = boot.presets || [];
        state.flywheel = boot.flywheel;
        if (boot.eval && boot.eval.scenario_id) {
          state.activeScenarioId = boot.eval.scenario_id;
        }
        renderScenarioButtons();
        if (boot.eval) updateDashboard(boot.eval);
        setStatus("Ready. Routing is computed from the payload, not from the scenario label.");
      } catch (e) {
        setStatus("Boot payload parse failed: " + e.message, true);
      }
    }

    ["slider-security", "slider-field", "slider-router", "slider-composite"].forEach(function (id) {
      var s = $(id);
      if (!s) return;
      s.addEventListener("input", function () {
        paintThresholdLabels(readThresholds());
        clearTimeout(state.debounceTimer);
        state.debounceTimer = setTimeout(syncThresholds, 220);
      });
    });

    $("run-eval-btn").addEventListener("click", evaluateCurrent);
    $("context-editor").addEventListener("keydown", function (ev) {
      if ((ev.metaKey || ev.ctrlKey) && ev.key === "Enter") {
        ev.preventDefault();
        evaluateCurrent();
      }
    });

    $("apply-recommended").addEventListener("click", function () {
      var f = state.flywheel;
      if (!f) return;
      $("slider-security").value = f.recommended_block_prob;
      $("slider-field").value = f.recommended_verify_min;
      $("slider-router").value = f.recommended_act_gate;
      $("slider-composite").value = f.recommended_composite;
      syncThresholds();
    });

    $("key-form").addEventListener("submit", function (ev) {
      ev.preventDefault();
      var input = $("api-key-input");
      var keyVal = input ? input.value.trim() : "";
      fetch("/api/key", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ api_key: keyVal })
      })
        .then(function (r) { return r.json(); })
        .then(function (res) {
          if (input) input.value = "";
          updateModePill(res.mode, res.has_api_key, false);
          setStatus(res.has_api_key ? "Live key stored in memory. Re-evaluating…" : "Key cleared. Simulator mode.");
          evaluateCurrent();
        });
    });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
