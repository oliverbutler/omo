document.addEventListener('DOMContentLoaded', () => {
  const trips = JSON.parse(document.getElementById('jsonData').textContent);

  console.log('maps.js: trips', trips);

  let currentTripIndex = trips.length - 1;
  let currentTrip = trips[currentTripIndex];
  let map, elevationChart;

  mapboxgl.accessToken =
    'pk.eyJ1Ijoib2xpdmVyYnV0bGVyIiwiYSI6ImNsZ3NpZmlvazAxb2Mzc281dXJvb20weGgifQ.yov1u2Efo_v7ImCH2o9pGg';

  let markers = [];

  const trailColors = [
    '#FF0000', // Red
    '#FF4500', // OrangeRed
    '#FF8C00', // DarkOrange
    '#FFA500', // Orange
    '#9400D3', // DarkViolet
    '#FF1493', // DeepPink
    '#00CED1', // DarkTurquoise
    '#FF69B4', // HotPink
    '#1E90FF', // DodgerBlue
    '#32CD32', // LimeGreen
  ];

  mapboxgl.config.ENABLE_EVENT_LOGGING = false;

  map = new mapboxgl.Map({
    container: 'map',
    style: 'mapbox://styles/oliverbutler/cllz4ea1c00n701pbbqah10qo',
    center: [-2.9263, 54.5441],
    zoom: 12,
    pitch: 60,
    bearing: 30,
    trackUserLocation: false,
    collectResourceTiming: false,
  });

  function initializeTripSelector() {
    const select = document.getElementById('trip-select');
    trips.forEach((trip, index) => {
      const option = document.createElement('option');
      option.value = index;
      option.textContent = trip.name;
      select.appendChild(option);
    });
    select.addEventListener('change', (e) => {
      currentTripIndex = parseInt(e.target.value);
      currentTrip = trips[currentTripIndex];
      updateMap();
      updateElevationChart();
    });
  }

  function getMostProminentTrip() {
    const bounds = map.getBounds();
    const visibleTrips = trips.map((trip, index) => {
      let visiblePoints = 0;
      trip.events.forEach((event) => {
        if (event.type === 'hike') {
          event.trackPointsLowRes.forEach((point) => {
            if (bounds.contains([point.lon, point.lat])) {
              visiblePoints++;
            }
          });
        }
      });
      return { index, visiblePoints };
    });

    const tripsWithVisiblePoints = visibleTrips.filter(
      (trip) => trip.visiblePoints > 0,
    );

    if (tripsWithVisiblePoints.length === 1) {
      return tripsWithVisiblePoints[0].index;
    }

    return null; // Return null if more than one trip is visible or no trips are visible
  }

  function updateCurrentTrip() {
    const newTripIndex = getMostProminentTrip();
    if (newTripIndex !== null && newTripIndex !== currentTripIndex) {
      console.log('Auto-switching to trip', newTripIndex);
      currentTripIndex = newTripIndex;
      currentTrip = trips[currentTripIndex];
      document.getElementById('trip-select').value = currentTripIndex;
      updateMap(false);
      updateElevationChart();
    }
  }

  function updateMap(fitBounds = true) {
    // Clear existing layers and sources
    for (let i = 0; i < trips.length; i++) {
      if (map.getLayer(`hike-tracks-${i}`)) map.removeLayer(`hike-tracks-${i}`);
      if (map.getSource(`hike-tracks-${i}`))
        map.removeSource(`hike-tracks-${i}`);
    }
    if (map.getLayer('camp-locations')) map.removeLayer('camp-locations');
    if (map.getSource('camp-locations')) map.removeSource('camp-locations');

    // Remove existing markers
    markers.forEach((marker) => marker.remove());
    markers = [];

    // Add all hike tracks
    trips.forEach((trip, tripIndex) => {
      const hikeFeatures = trip.events
        .filter((event) => event.type === 'hike')
        .map((hike) => {
          const isCurrentTrip = tripIndex === currentTripIndex;
          const trackPoints = isCurrentTrip
            ? hike.trackPoints
            : hike.trackPointsLowRes;

          return {
            type: 'Feature',
            properties: {},
            geometry: {
              type: 'LineString',
              coordinates: trackPoints.map((point) => [
                point.lon,
                point.lat,
                point.ele,
              ]),
            },
          };
        });

      map.addSource(`hike-tracks-${tripIndex}`, {
        type: 'geojson',
        data: {
          type: 'FeatureCollection',
          features: hikeFeatures,
        },
      });

      map.addLayer({
        id: `hike-tracks-${tripIndex}`,
        type: 'line',
        source: `hike-tracks-${tripIndex}`,
        layout: {
          'line-join': 'round',
          'line-cap': 'round',
        },
        paint: {
          'line-color': trailColors[tripIndex % trailColors.length],
          'line-width': tripIndex === currentTripIndex ? 3 : 2,
          'line-opacity': tripIndex === currentTripIndex ? 1 : 0.7,
        },
      });
    });

    // Add all camp locations
    const allCampFeatures = trips.flatMap((trip, tripIndex) =>
      trip.events
        .filter((event) => event.type === 'camp')
        .map((camp) => ({
          type: 'Feature',
          properties: { name: camp.name, tripIndex: tripIndex },
          geometry: {
            type: 'Point',
            coordinates: [camp.lon, camp.lat],
          },
        })),
    );

    map.addSource('camp-locations', {
      type: 'geojson',
      data: {
        type: 'FeatureCollection',
        features: allCampFeatures,
      },
    });

    map.addLayer({
      id: 'camp-locations',
      type: 'circle',
      source: 'camp-locations',
      paint: {
        'circle-radius': 6,
        'circle-color': '#4CAF50',
      },
    });

    if (fitBounds) {
      // Fit map to bounds of current trip
      const bounds = new mapboxgl.LngLatBounds();
      currentTrip.events.forEach((event) => {
        if (event.type === 'hike') {
          event.trackPointsLowRes.forEach((point) => {
            bounds.extend([point.lon, point.lat]);
          });
        } else if (event.type === 'camp') {
          bounds.extend([event.lon, event.lat]);
        }
      });

      map.fitBounds(bounds, {
        padding: 50,
        pitch: 60,
        bearing: 30,
        duration: 2000,
      });
    }

    // Add markers for current trip
    const start = currentTrip.events.find((event) => event.type === 'hike')
      .trackPoints[0];
    markers.push(
      new mapboxgl.Marker({ color: '#008000' })
        .setLngLat([start.lon, start.lat])
        .addTo(map),
    );

    const lastHike = currentTrip.events
      .filter((event) => event.type === 'hike')
      .pop();
    const end = lastHike.trackPoints[lastHike.trackPoints.length - 1];
    markers.push(
      new mapboxgl.Marker({ color: '#FF0000' })
        .setLngLat([end.lon, end.lat])
        .addTo(map),
    );

    // Add markers for all camps
    allCampFeatures.forEach((camp) => {
      const marker = new mapboxgl.Marker({ color: '#0000FF' })
        .setLngLat(camp.geometry.coordinates)
        .setPopup(
          new mapboxgl.Popup().setHTML(`<h3>${camp.properties.name}</h3>`),
        )
        .addTo(map);

      marker.getElement().addEventListener('click', () => {
        currentTripIndex = camp.properties.tripIndex;
        currentTrip = trips[currentTripIndex];
        document.getElementById('trip-select').value = currentTripIndex;
        updateMap();
        updateElevationChart();
      });

      markers.push(marker);
    });
  }

  function updateElevationChart() {
    const ctx = document.getElementById('elevation-chart').getContext('2d');

    let allPoints = [];
    let cumulativeDistance = 0;

    currentTrip.events.forEach((event, index) => {
      if (event.type === 'hike') {
        event.trackPointsLowRes.forEach((point, i) => {
          const prevPoint = i > 0 ? event.trackPointsLowRes[i - 1] : null;
          let grade = 0;
          if (prevPoint) {
            const elevationChange = point.ele - prevPoint.ele; // in meters
            const distance = (point.cumDistance - prevPoint.cumDistance) * 1000; // convert km to meters
            grade = distance > 0 ? elevationChange / distance : 0;
          }
          allPoints.push({
            x: cumulativeDistance + point.cumDistance,
            y: point.ele,
            lon: point.lon,
            lat: point.lat,
            grade: grade,
          });
        });
        cumulativeDistance +=
          event.trackPoints[event.trackPoints.length - 1].cumDistance;
      } else if (event.type === 'camp') {
        allPoints.push({
          x: cumulativeDistance,
          y: event.alt,
          lon: event.lon,
          lat: event.lat,
          isCamp: true,
          name: event.name,
          grade: 0,
        });
      }
    });

    const maxDistance = Math.ceil(cumulativeDistance);
    const labels = Array.from({ length: maxDistance + 1 }, (_, i) => i);

    if (elevationChart) {
      elevationChart.destroy();
    }

    const GRADE_STEEP = 0.1;
    const GRADE_MODERATE = 0.05;

    elevationChart = new Chart(ctx, {
      type: 'line',
      data: {
        labels: labels,
        datasets: [
          {
            label: 'Grade - Steep',
            data: allPoints.map((point) => ({
              x: point.x,
              y: point.grade > GRADE_STEEP ? point.y : null,
            })),
            backgroundColor: 'rgba(255, 0, 0, 0.3)',
            borderWidth: 0,
            fill: true,
            tension: 0,
            pointRadius: 0,
          },
          {
            label: 'Grade - Moderate',
            data: allPoints.map((point) => ({
              x: point.x,
              y:
                point.grade > GRADE_MODERATE && point.grade <= GRADE_STEEP
                  ? point.y
                  : null,
            })),
            backgroundColor: 'rgba(255, 255, 0, 0.3)',
            borderWidth: 0,
            fill: true,
            tension: 0,
            pointRadius: 0,
          },
          {
            label: 'Elevation',
            data: allPoints.map((point) => ({ x: point.x, y: point.y })),
            borderColor: 'rgba(0, 0, 0, 0.5)',
            borderWidth: 2,
            fill: false,
            tension: 0.1,
            pointRadius: 0,
          },
          {
            label: 'Camps',
            data: allPoints
              .filter((point) => point.isCamp)
              .map((point) => ({ x: point.x, y: point.y })),
            backgroundColor: '#4CAF50',
            borderColor: '#4CAF50',
            borderWidth: 2,
            pointRadius: 5,
            type: 'scatter',
          },
        ],
      },
      options: {
        responsive: true,
        maintainAspectRatio: false,
        scales: {
          x: {
            type: 'linear',
            title: {
              display: true,
              text: 'Distance (km)',
            },
            ticks: {
              stepSize: 1,
              callback: function (value, index, values) {
                return value.toFixed(0);
              },
            },
            min: 0,
            max: maxDistance,
          },
          y: {
            title: {
              display: true,
              text: 'Elevation (m)',
            },
          },
        },
        plugins: {
          tooltip: {
            callbacks: {
              title: function (context) {
                const point = allPoints[context[0].dataIndex];
                if (point.isCamp) {
                  return `Camp: ${point.name}`;
                }
                return `Distance: ${point.x.toFixed(2)} km`;
              },
              label: function (context) {
                const point = allPoints[context.dataIndex];
                let label = `Elevation: ${point.y.toFixed(0)} m`;
                if (!point.isCamp) {
                  label += `\nGrade: ${(point.grade * 100).toFixed(1)}%`;
                }
                return label;
              },
            },
          },
          legend: {
            display: false,
          },
        },
        onClick: (event, elements) => {
          if (elements.length > 0) {
            const index = elements[0].index;
            const point = allPoints[index];
            map.flyTo({
              center: [point.lon, point.lat],
              zoom: 14,
              pitch: 60,
              bearing: 30,
              duration: 1000,
            });
          }
        },
      },
    });
  }

  map.on('load', () => {
    map.resize();

    initializeTripSelector();
    updateMap();
    updateElevationChart();

    // Layer switcher functionality
    document.getElementById('map-button').addEventListener('click', () => {
      map.setStyle('mapbox://styles/oliverbutler/cllz4ea1c00n701pbbqah10qo');
    });

    document
      .getElementById('satellite-button')
      .addEventListener('click', () => {
        map.setStyle('mapbox://styles/oliverbutler/cm0cp3xo200tc01qt51wieggw');
      });

    map.on('style.load', () => {
      updateMap();
    });
  });

  map.on('moveend', updateCurrentTrip);

  // Add keyboard navigation
  document.addEventListener('keydown', (e) => {
    if (e.key === 'h') {
      currentTripIndex = (currentTripIndex - 1 + trips.length) % trips.length;
    } else if (e.key === 'l') {
      currentTripIndex = (currentTripIndex + 1) % trips.length;
    } else if (e.key === 'n') {
      map.easeTo({ bearing: 0 });
    } else if (e.key === 's') {
      document.getElementById('satellite-button').click();
    } else if (e.key === 'm') {
      document.getElementById('map-button').click();
    } else if (e.key === 'c') {
      // Copy current lat/lon under cursor to clipboard, in the YAML format
      const center = map.getCenter();
      const lat = center.lat.toFixed(6);
      const lon = center.lng.toFixed(6);
      const ele = map.queryTerrainElevation(center, { exaggerated: false });
      const yaml = `  lat: ${lat}\n  lon: ${lon}\n  ele: ${ele ? ele.toFixed(1) : 'N/A'}`;
      navigator.clipboard
        .writeText(yaml)
        .then(() => {
          alert('Location copied to clipboard!');
        })
        .catch((err) => {
          console.error('Failed to copy location: ', err);
          alert('Failed to copy location. See console for details.');
        });
      return;
    }
    currentTrip = trips[currentTripIndex];
    document.getElementById('trip-select').value = currentTripIndex;
    updateMap();
    updateElevationChart();
  });
});
