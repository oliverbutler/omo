function getColumnCount() {
  const width = window.innerWidth;
  if (width >= 1024) return 4;
  if (width >= 640) return 3;
  return 2;
}

function layoutMasonry(isInitialLoad = false) {
  console.log("Running layout masonry"); // Debug log
  const grid = document.getElementById("masonry-grid");
  if (!grid) return;

  const gap = 16;
  const columnCount = getColumnCount();
  const columnHeights = new Array(columnCount).fill(0);
  const gridWidth = grid.offsetWidth;
  const columnWidth = (gridWidth - gap * (columnCount - 1)) / columnCount;

  const items = Array.from(grid.getElementsByClassName("photo-item"));

  // Reset grid height
  grid.style.height = "0px";

  items.forEach((item) => {
    const img = item.querySelector("img");
    if (!img) return;

    // Get aspect ratio from the image
    const aspectRatio = parseFloat(img.style.aspectRatio);
    const itemHeight = columnWidth / aspectRatio;

    // Find the column with the smallest height
    let smallestHeight = Math.min(...columnHeights);
    let columnIndex = columnHeights.indexOf(smallestHeight);

    // Position the item
    const xPos = columnIndex * (columnWidth + gap);
    const yPos = columnHeights[columnIndex];

    item.style.transform = `translate(${xPos}px, ${yPos}px)`;
    item.style.width = `${columnWidth}px`;

    // If it's the initial load, set position immediately without animation
    if (isInitialLoad) {
      requestAnimationFrame(() => {
        item.style.opacity = "1";
        // Delay adding the initialized class to prevent initial transform animation
        setTimeout(() => {
          item.classList.add("initialized");
        }, 500); // After fade-in completes
      });
    } else {
      item.style.opacity = "1";
    }

    // Update the column height
    columnHeights[columnIndex] += itemHeight + gap;

    // Update grid height
    const maxHeight = Math.max(...columnHeights);
    grid.style.height = `${maxHeight}px`;
  });
}

// Initial layout and image load handling
document.addEventListener("DOMContentLoaded", () => {
  // Do initial layout with special flag
  layoutMasonry(true);

  // Set up image load handlers
  const images = document.querySelectorAll(".photo-item img");
  let loadedImages = 0;

  images.forEach((img) => {
    if (img.complete) {
      loadedImages++;
    } else {
      img.addEventListener("load", () => {
        loadedImages++;
        if (loadedImages === images.length) {
          layoutMasonry();
        }
      });
    }
  });

  // If all images are already loaded, run layout again
  if (loadedImages === images.length) {
    layoutMasonry();
  }
});

// Reflow on window resize
let resizeTimeout;
window.addEventListener("resize", () => {
  clearTimeout(resizeTimeout);
  resizeTimeout = setTimeout(layoutMasonry, 100);
});
