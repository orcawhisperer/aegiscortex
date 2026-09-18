(function () {
  "use strict";

  var state = {
    presets: [],
    activeScenarioId: "rag_verified_fastpath",
    thresholds: {
      security_gate_confidence: 0.85,
      field_verify_confidence: 0.90,
      router_confidence: 0.80,
      composite_pass_threshold: 68.0
    }
  };

  function el(tag, className, text) {
    var node = document.createElement(tag);
    if (className) {
      node.className = className;
    }
    if (text !== undefined && text !== null) {
      node.textContent = String(text);
    }
    return node;
  }

  function setStatus(msg, isError) {
    var banner = document.getElementById("status-banner");
    if (!banner) return;
    banner.textContent = msg;
    banner.className = isError
      ? "mt-2.5 px-3 py-2 rounded-lg bg-rose-950/50 border border-rose-500/50 font-mono text-xs text-rose-300"
      : "mt-2.5 px-3 py-2 rounded-lg bg-slate-950 border border-slate-800/80 font-mono text-xs text-sky-300";
  }

  function updateModePill(mode, hasKey) {
    var pill = document.getElementById("mode-indicator");
    var textNode = document.getElementById("mode-text");
    if (!pill || !textNode) return;
    if (hasKey || mode === "LIVE_TYPESAFE_API") {
      pill.className = "mode-pill mode-live flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-mono font-bold border";
      textNode.textContent = "LIVE TYPESAFE API (JEV-1.13)";
    } else {
      pill.className = "mode-pill mode-sim flex items-center gap-1.5 px-3 py-1 rounded-full text-xs font-mono font-bold border";
      textNode.textContent = "CALIBRATED JEV-1.13 RLCD ENGINE";
    }
  }

  function renderScenarioButtons() {
    var container = document.getElementById("scenario-list");
    if (!container || !state.presets.length) return;
    var nodes = [];

    for (var i = 0; i < state.presets.length; i++) {
      var p = state.presets[i];
      var isActive = p.id === state.activeScenarioId;
      var btnClass = isActive
        ? "scenario-btn active w-full text-left p-3 rounded-lg bg-sky-950/30 border border-sky-400 transition"
        : "scenario-btn w-full text-left p-3 rounded-lg bg-slate-950/80 border border-slate-800 hover:border-sky-400/60 transition";

      var btn = el("button", btnClass);
      btn.type = "button";
      btn.setAttribute("data-scenario-id", p.id);

      var topRow = el("div", "flex items-center justify-between mb-1");
      var titleSpan = el("span", "font-bold text-xs text-slate-100", p.title);
      var badgeSpan = el("span", "font-mono text-[10px] px-2 py-0.5 rounded bg-sky-500/15 text-sky-300", p.badge);
      topRow.appendChild(titleSpan);
      topRow.appendChild(badgeSpan);

      var descP = el("p", "text-xs text-slate-400 leading-relaxed", p.description);
      btn.appendChild(topRow);
      btn.appendChild(descP);

      (function (preset) {
        btn.addEventListener("click", function () {
          state.activeScenarioId = preset.id;
          var editor = document.getElementById("context-editor");
          if (editor) {
            editor.value = JSON.stringify(preset.context, null, 2);
          }
          renderScenarioButtons();
          evaluateCurrent(preset.id);
        });
      })(p);

      nodes.push(btn);
    }

    container.replaceChildren.apply(container, nodes);
  }

  function renderFieldVerifications(fields) {
    var grid = document.getElementById("field-verifications-grid");
    if (!grid) return;
    var cards = [];

    for (var i = 0; i < fields.length; i++) {
      var f = fields[i];
      var cardClass = f.verified
        ? "field-card field-verified bg-slate-950/90 border border-emerald-500/40 rounded-lg p-3.5"
        : "field-card field-failed bg-rose-950/20 border border-rose-500/60 rounded-lg p-3.5";

      var card = el("div", cardClass);

      var top = el("div", "flex items-center justify-between mb-1.5");
      var nameSpan = el("span", "font-mono text-xs font-bold text-slate-100", f.field_name);
      var badge = el(
        "span",
        f.verified
          ? "font-mono text-[10px] font-bold px-2 py-0.5 rounded bg-emerald-500/20 text-emerald-300"
          : "font-mono text-[10px] font-bold px-2 py-0.5 rounded bg-rose-500/20 text-rose-300",
        f.verified ? "LOCKED_OK" : "ESCALATE_FIELD"
      );
      top.appendChild(nameSpan);
      top.appendChild(badge);

      var probs = el(
        "div",
        "font-mono text-[11px] text-slate-400 mb-2",
        "P(YES)=" + (f.yes_prob * 100).toFixed(1) + "% | P(NO)=" + (f.no_prob * 100).toFixed(1) + "% | conf=" + f.confidence.toFixed(2)
      );

      var barWrap = el("div", "w-full h-1.5 bg-slate-900 rounded-full overflow-hidden mb-2");
      var barFill = el(
        "div",
        f.verified ? "h-full bg-emerald-400" : "h-full bg-rose-500"
      );
      barFill.style.width = Math.max(4, Math.min(100, Math.round(f.yes_prob * 100))) + "%";
      barWrap.appendChild(barFill);

      var actionNote = el(
        "div",
        f.verified ? "font-mono text-[11px] text-emerald-300" : "font-mono text-[11px] text-rose-300 font-semibold",
        f.action
      );

      card.appendChild(top);
      card.appendChild(probs);
      card.appendChild(barWrap);
      card.appendChild(actionNote);
      cards.push(card);
    }

    grid.replaceChildren.apply(grid, cards);
  }

  function renderQuestionsMatrix(questions) {
    var matrix = document.getElementById("questions-matrix");
    if (!matrix) return;
    var cards = [];

    for (var i = 0; i < questions.length; i++) {
      var q = questions[i];
      var card = el("div", "bg-slate-950/85 border border-slate-800 rounded-lg p-3.5");

      var top = el("div", "flex items-center justify-between gap-2 mb-1.5");
      var idSpan = el("span", "font-mono text-xs font-bold text-sky-400", q.id);

      var badgeWrap = el("div", "flex items-center gap-1.5");
      var stageBadge = el("span", "font-mono text-[10px] px-2 py-0.5 rounded bg-slate-800 text-slate-300", q.stage);
      var typeBadge = el("span", "font-mono text-[10px] px-2 py-0.5 rounded bg-purple-500/15 text-purple-300", q.type);
      badgeWrap.appendChild(stageBadge);
      badgeWrap.appendChild(typeBadge);

      top.appendChild(idSpan);
      top.appendChild(badgeWrap);

      var promptP = el("p", "text-xs text-slate-400 mb-2 leading-snug", q.prompt);

      var choiceRow = el("div", "flex items-center justify-between font-mono text-xs mb-2");
      var selectedText = el("span", "text-slate-200 font-bold", "Choice: " + q.selected_choice + " (" + (q.top_probability * 100).toFixed(1) + "%)");
      var confText = el(
        "span",
        q.auto_executable ? "text-emerald-400" : "text-amber-400",
        "conf=" + q.confidence.toFixed(2) + " | rel=" + q.relevance.toFixed(2)
      );
      choiceRow.appendChild(selectedText);
      choiceRow.appendChild(confText);

      var barWrap = el("div", "w-full h-1.5 bg-slate-900 rounded-full overflow-hidden mb-1.5");
      var barFill = el(
        "div",
        q.auto_executable ? "h-full bg-gradient-to-r from-sky-400 to-emerald-400" : "h-full bg-amber-400"
      );
      barFill.style.width = Math.max(4, Math.min(100, Math.round(q.top_probability * 100))) + "%";
      barWrap.appendChild(barFill);

      var reasonText = el("div", "font-mono text-[10px] text-slate-500", q.route_reason);

      card.appendChild(top);
      card.appendChild(promptP);
      card.appendChild(choiceRow);
      card.appendChild(barWrap);
      card.appendChild(reasonText);

      cards.push(card);
    }

    matrix.replaceChildren.apply(matrix, cards);
  }

  function updateDashboard(res) {
    if (!res) return;

    updateModePill(res.mode, res.mode === "LIVE_TYPESAFE_API");

    var kpiLat = document.getElementById("kpi-latency");
    if (kpiLat && res.cascade) kpiLat.textContent = res.cascade.aegis_control_latency_ms.toFixed(0) + " ms";

    var kpiCost = document.getElementById("kpi-jev-cost");
    if (kpiCost && res.cascade) kpiCost.textContent = "$" + res.cascade.aegis_control_cost_usd.toFixed(6);

    var kpiSavings = document.getElementById("kpi-savings-pct");
    if (kpiSavings && res.cascade) kpiSavings.textContent = res.cascade.cost_savings_percent.toFixed(1) + "%";

    var kpiSavingsSub = document.getElementById("kpi-savings-sub");
    if (kpiSavingsSub && res.cascade) {
      kpiSavingsSub.textContent = "vs $" + res.cascade.naive_frontier_cost_usd.toFixed(6) + " Naive Frontier";
    }

    var kpiComp = document.getElementById("kpi-composite");
    if (kpiComp) kpiComp.textContent = res.composite_score.toFixed(1) + " / 100";

    var kpiRoute = document.getElementById("kpi-route-tier");
    if (kpiRoute) kpiRoute.textContent = res.final_route_tier;

    if (res.flywheel) {
      var kpiFly = document.getElementById("kpi-flywheel");
      if (kpiFly) kpiFly.textContent = res.flywheel.distilled_golden_examples + " Distilled";
      var kpiFlySub = document.getElementById("kpi-flywheel-sub");
      if (kpiFlySub) {
        kpiFlySub.textContent = res.flywheel.tier0_blocked_or_auto + " Blocked/Auto \u2022 " + res.flywheel.tier2_surgical_escalations + " Surgical Repairs";
      }
    }

    var tierTitle = document.getElementById("decision-tier-title");
    if (tierTitle) tierTitle.textContent = res.final_route_tier;

    var tierBadge = document.getElementById("decision-badge");
    if (tierBadge) {
      tierBadge.textContent = res.final_route_tier;
      if (res.final_route_tier === "TIER_0_BLOCK") {
        tierBadge.className = "tier-pill tier-block font-mono text-xs font-bold px-3 py-1.5 rounded-lg";
      } else if (res.final_route_tier.indexOf("TIER_2") === 0) {
        tierBadge.className = "tier-pill tier-repair font-mono text-xs font-bold px-3 py-1.5 rounded-lg";
      } else {
        tierBadge.className = "tier-pill tier-fastpath font-mono text-xs font-bold px-3 py-1.5 rounded-lg";
      }
    }

    var rationale = document.getElementById("decision-rationale");
    if (rationale) rationale.textContent = res.final_decision;

    if (res.cascade) {
      var nc = document.getElementById("arch-naive-cost");
      var nl = document.getElementById("arch-naive-lat");
      var lc = document.getElementById("arch-legacy-cost");
      var ll = document.getElementById("arch-legacy-lat");
      var ac = document.getElementById("arch-aegis-cost");
      var al = document.getElementById("arch-aegis-lat");
      if (nc) nc.textContent = "$" + res.cascade.naive_frontier_cost_usd.toFixed(6);
      if (nl) nl.textContent = res.cascade.naive_frontier_latency_ms.toFixed(0) + " ms";
      if (lc) lc.textContent = "$" + res.cascade.legacy_router_cost_usd.toFixed(6);
      if (ll) ll.textContent = res.cascade.legacy_router_latency_ms.toFixed(0) + " ms";
      if (ac) ac.textContent = "$" + res.cascade.aegis_blended_cost_usd.toFixed(6);
      if (al) al.textContent = res.cascade.aegis_total_latency_ms.toFixed(0) + " ms";
    }

    renderFieldVerifications(res.field_verifications || []);
    renderQuestionsMatrix(res.questions || []);
  }

  function evaluateCurrent(scenarioIdOverride) {
    var editor = document.getElementById("context-editor");
    var parsedContext = {};
    if (editor && editor.value.trim() !== "") {
      try {
        parsedContext = JSON.parse(editor.value);
      } catch (e) {
        setStatus("Invalid JSON in Shared KV Context Payload: " + e.message, true);
        return;
      }
    }

    setStatus("Executing 11-question parallel fan-out via typesafe-sdk-go...", false);

    fetch("/api/evaluate", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        scenario_id: scenarioIdOverride || state.activeScenarioId,
        context: parsedContext
      })
    })
      .then(function (r) { return r.json(); })
      .then(function (data) {
        updateDashboard(data);
        setStatus(
          "Completed 11-Q Fan-Out in " + data.latency_ms.toFixed(1) + "ms | Route: " + data.final_route_tier + " | Savings: " + data.cascade.cost_savings_percent.toFixed(1) + "%",
          false
        );
      })
      .catch(function (err) {
        setStatus("Evaluation failed: " + err.message, true);
      });
  }

  function syncThresholds() {
    var sSec = document.getElementById("slider-security");
    var sField = document.getElementById("slider-field");
    var sRoute = document.getElementById("slider-router");
    var sComp = document.getElementById("slider-composite");

    var payload = {
      security_gate_confidence: parseFloat(sSec.value),
      field_verify_confidence: parseFloat(sField.value),
      router_confidence: parseFloat(sRoute.value),
      composite_pass_threshold: parseFloat(sComp.value)
    };

    document.getElementById("val-security").textContent = payload.security_gate_confidence.toFixed(2);
    document.getElementById("val-field").textContent = payload.field_verify_confidence.toFixed(2);
    document.getElementById("val-router").textContent = payload.router_confidence.toFixed(2);
    document.getElementById("val-composite").textContent = payload.composite_pass_threshold.toFixed(1);

    fetch("/api/thresholds", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(payload)
    })
      .then(function (r) { return r.json(); })
      .then(function () {
        evaluateCurrent(state.activeScenarioId);
      });
  }

  function init() {
    var sliders = ["slider-security", "slider-field", "slider-router", "slider-composite"];
    for (var i = 0; i < sliders.length; i++) {
      var s = document.getElementById(sliders[i]);
      if (s) {
        s.addEventListener("input", syncThresholds);
      }
    }

    var runBtn = document.getElementById("run-eval-btn");
    if (runBtn) {
      runBtn.addEventListener("click", function () {
        evaluateCurrent("");
      });
    }

    var keyBtn = document.getElementById("save-key-btn");
    if (keyBtn) {
      keyBtn.addEventListener("click", function () {
        var input = document.getElementById("api-key-input");
        var keyVal = input ? input.value.trim() : "";
        fetch("/api/key", {
          method: "POST",
          headers: { "Content-Type": "application/json" },
          body: JSON.stringify({ api_key: keyVal })
        })
          .then(function (r) { return r.json(); })
          .then(function (res) {
            if (input) input.value = "";
            updateModePill(res.mode, res.has_api_key);
            setStatus(
              res.has_api_key
                ? "Configured live TYPESAFE_API_KEY in server memory. Re-evaluating..."
                : "Cleared TYPESAFE_API_KEY; switched to Calibrated Jev-1.13 RLCD Engine.",
              false
            );
            evaluateCurrent(state.activeScenarioId);
          });
      });
    }

    fetch("/api/state")
      .then(function (r) { return r.json(); })
      .then(function (data) {
        state.presets = data.presets || [];
        if (state.presets.length > 0) {
          state.activeScenarioId = "rag_verified_fastpath";
          for (var i = 0; i < state.presets.length; i++) {
            if (state.presets[i].id === state.activeScenarioId) {
              var editor = document.getElementById("context-editor");
              if (editor) {
                editor.value = JSON.stringify(state.presets[i].context, null, 2);
              }
            }
          }
        }
        renderScenarioButtons();
        evaluateCurrent(state.activeScenarioId);
      });
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", init);
  } else {
    init();
  }
})();
