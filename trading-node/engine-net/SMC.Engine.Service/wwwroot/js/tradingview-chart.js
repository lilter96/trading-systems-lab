(function () {
  'use strict';

  const LWC = window.LightweightCharts;

  class PoiZoneRenderer {
    constructor() {
      this._zones = [];
    }

    setZones(zones) {
      this._zones = zones;
    }

    draw(target) {
      target.useMediaCoordinateSpace(({ context: ctx, mediaSize }) => {
        const zones = this._zones;
        if (!zones.length) return;

        for (let i = 0; i < zones.length; i++) {
          const z = zones[i];
          if (z.yTop == null || z.yBottom == null) continue;

          const yTop = Math.min(z.yTop, z.yBottom);
          const yBottom = Math.max(z.yTop, z.yBottom);
          const height = yBottom - yTop;
          if (height < 1) continue;

          ctx.fillStyle = z.fillColor;
          ctx.fillRect(0, yTop, mediaSize.width, height);

          ctx.strokeStyle = z.borderColor;
          ctx.lineWidth = 1.5;
          ctx.setLineDash(z.lineDash ? [4, 3] : []);
          ctx.beginPath();
          ctx.moveTo(0, yTop);
          ctx.lineTo(mediaSize.width, yTop);
          ctx.stroke();
          ctx.beginPath();
          ctx.moveTo(0, yBottom);
          ctx.lineTo(mediaSize.width, yBottom);
          ctx.stroke();
          ctx.setLineDash([]);
        }
      });
    }
  }

  class PoiZonePaneView {
    constructor(renderer) {
      this._renderer = renderer;
    }
    renderer() { return this._renderer; }
  }

  class PoiZonePrimitive {
    constructor(series) {
      this._series = series;
      this._zones = [];
      this._renderer = new PoiZoneRenderer();
      this._view = new PoiZonePaneView(this._renderer);
    }

    updateZones(zones) {
      this._zones = zones;
      this._updateRenderer();
    }

    _updateRenderer() {
      const series = this._series;
      if (!series || !this._zones.length) {
        this._renderer.setZones([]);
        return;
      }

      const mapped = [];
      for (let i = 0; i < this._zones.length; i++) {
        const z = this._zones[i];
        const yTop = series.priceToCoordinate(z.upper);
        const yBottom = series.priceToCoordinate(z.lower);
        if (yTop === null || yBottom === null) continue;
        mapped.push({
          yTop,
          yBottom,
          fillColor: z.fillColor,
          borderColor: z.borderColor,
          lineDash: z.lineDash,
        });
      }
      this._renderer.setZones(mapped);
    }

    paneViews() {
      return [this._view];
    }

    updateAllViews() {
      this._updateRenderer();
    }

    attached(_pane) {}
    detached() {}
  }

  class ChartController {
    constructor() {
      this._chart = null;
      this._series = null;
      this._poiPrimitive = null;
      this._markerPrimitive = null;
      this._containerId = null;
    }

    init(containerId) {
      if (this._chart) {
        console.warn('[Chart] Already initialized, reusing');
        return;
      }

      const container = document.getElementById(containerId);
      if (!container) {
        console.error('[Chart] Container not found:', containerId);
        return;
      }

      this._containerId = containerId;

      this._chart = LWC.createChart(container, {
        layout: {
          background: { color: '#ffffff' },
          textColor: '#6c757d',
        },
        grid: {
          vertLines: { color: '#e9ecef' },
          horzLines: { color: '#e9ecef' },
        },
        crosshair: {
          mode: LWC.CrosshairMode.Normal,
        },
        rightPriceScale: {
          borderColor: '#dee2e6',
          scaleMargins: { top: 0.1, bottom: 0.15 },
        },
        timeScale: {
          borderColor: '#dee2e6',
          timeVisible: true,
          secondsVisible: false,
          rightOffset: 10,
        },
        handleScroll: { vertTouchDrag: true },
        autoSize: true,
      });

      this._series = this._chart.addSeries(LWC.CandlestickSeries, {
        upColor: '#198754',
        downColor: '#dc3545',
        borderUpColor: '#198754',
        borderDownColor: '#dc3545',
        wickUpColor: '#198754',
        wickDownColor: '#dc3545',
      });

      this._poiPrimitive = new PoiZonePrimitive(this._series);
      this._chart.panes()[0].attachPrimitive(this._poiPrimitive);

      this._chart.timeScale().fitContent();
    }

    setCandles(candles, fitView) {
      if (!this._series) return;
      this._series.setData(candles);
      if (fitView && this._chart) this._chart.timeScale().fitContent();
    }

    updateCandle(candle) {
      if (!this._series) return;
      this._series.update(candle);
    }

    setZones(zones) {
      if (!this._poiPrimitive) return;
      this._poiPrimitive.updateZones(zones || []);
    }

    setMarkers(markers) {
      if (!this._series) return;
      if (this._markerPrimitive) {
        this._markerPrimitive.setMarkers(markers || []);
      } else if (markers && markers.length) {
        this._markerPrimitive = LWC.createSeriesMarkers(this._series, markers);
      }
    }

    clearZones() {
      if (this._poiPrimitive) this._poiPrimitive.updateZones([]);
      if (this._markerPrimitive) this._markerPrimitive.setMarkers([]);
    }

    destroy() {
      this.clearZones();
      this._markerPrimitive = null;
      this._poiPrimitive = null;
      this._series = null;
      if (this._chart) {
        this._chart.remove();
        this._chart = null;
      }
    }
  }

  window.ChartController = ChartController;
  window.createChartController = function(containerId) {
    if (!window._chartController) {
      window._chartController = new ChartController();
    }
    window._chartController.init(containerId);
  };
})();
