document.addEventListener("DOMContentLoaded", function () {
  document.body.addEventListener("click", (e) => {
    // Check if the clicked element is a link
    const link = e.target.closest("a");
    if (!link) return;

    e.preventDefault();

    if (!document.startViewTransition) {
      return (window.location = link.href);
    }

    console.log("Starting transition");
    document.startViewTransition(() => {
      console.log("Transitioning to", link.href);
      window.location = link.href;
    });
  });
});

import * as blurhash from "https://cdn.jsdelivr.net/npm/blurhash@2.0.5/+esm";

function processBlurHashImages() {
  const images = document.querySelectorAll(
    "*[blur-hash]:not(.processed-blur-hash)",
  );
  images.forEach((img) => {
    const canvas = document.createElement("canvas");
    const blurHash = img.getAttribute("blur-hash");
    const width = parseInt(img.getAttribute("data-width"), 10);
    const height = parseInt(img.getAttribute("data-height"), 10);
    const aspectRatio = width / height;

    const canvasHeight = 4;
    const canvasWidth = Math.floor(aspectRatio * canvasHeight);

    canvas.width = canvasWidth;
    canvas.height = canvasHeight;

    // Decode and render BlurHash
    const pixels = blurhash.decode(blurHash, canvas.width, canvas.height);
    const ctx = canvas.getContext("2d");
    const imageData = ctx.createImageData(canvas.width, canvas.height);
    imageData.data.set(pixels);
    ctx.putImageData(imageData, 0, 0);

    // Set canvas as background
    img.style.backgroundImage = "url(" + canvas.toDataURL() + ")";
    img.style.backgroundSize = "cover";
    img.style.backgroundPosition = "center";

    // Remove BlurHash when image loads
    img.onload = function () {
      img.style.backgroundImage = "none";
    };

    // Mark this image as processed
    img.classList.add("processed-blur-hash");
  });
}

// Process BlurHash images on initial load
document.addEventListener("DOMContentLoaded", processBlurHashImages);

// Process BlurHash images after HTMX content swap
document.body.addEventListener("htmx:afterSwap", processBlurHashImages);

// Optional: Process BlurHash images after HTMX content load (if you're using hx-trigger="load" anywhere)
document.body.addEventListener("htmx:load", processBlurHashImages);
