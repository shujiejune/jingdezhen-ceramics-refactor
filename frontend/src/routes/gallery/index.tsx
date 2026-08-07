import { createFileRoute, Link } from "@tanstack/solid-router";
import { useQuery } from "@tanstack/solid-query";
import { For, Show, Suspense, createSignal, createEffect } from "solid-js";
import type { Component, JSX } from "solid-js";
import { Artwork } from "~/lib/types";
import { CaretLeft, CaretRight, Heart } from "~/components/icons/Phosphor";

// --- Mock User Data ---
// In a real app, this would come from an auth context.
const mockUser = {
  isLoggedIn: true,
};

// --- Mock API Fetching ---

const createMockArtworks = (category: string, count: number): Artwork[] => {
  return Array.from({ length: count }, (_, i) => {
    const id = i + 1 + count * Math.random();
    return {
      id: Math.floor(id),
      title: `${category} Vase No. ${i + 1}`,
      period: i % 2 === 0 ? "Ming Yongle" : "Qing Kangxi",
      thumbnailUrl: `https://placehold.co/400x600/e0f2fe/0891b2?text=${category.replace(" ", "+")}+${i + 1}`,
      category: category,
      isFavorite: mockUser.isLoggedIn && Math.random() > 0.5,
      favoriteCount: Math.floor(Math.random() * 150),
      noteCount: 0,
      createdAt: new Date().toISOString(),
    };
  });
};

const fetchArtworksByCategory = async (
  category: string,
): Promise<Artwork[]> => {
  console.log(`Fetching artworks for category: ${category}`);
  await new Promise((r) => setTimeout(r, 300 + Math.random() * 400));
  return createMockArtworks(category, 12);
};

// --- Route Definition ---

export const Route = createFileRoute("/gallery/")({
  component: GalleryPage,
});

// --- Main Page Component ---

function GalleryPage() {
  const categories = [
    "Blue-and-white",
    "Famille rose",
    "Rice pattern",
    "Colored glaze",
  ];

  return (
    <div class="container mx-auto px-4 py-8">
      <header class="text-center mb-12">
        <h1 class="text-4xl font-bold">Gallery</h1>
        <p class="text-lg text-gray-600 mt-2">
          Explore timeless masterpieces from renowned collections.
        </p>
      </header>

      <div class="space-y-16">
        <For each={categories}>
          {(category) => <ArtworkCarousel category={category} />}
        </For>
      </div>
    </div>
  );
}

// --- Sub-Components ---

const ArtworkCarousel: Component<{ category: string }> = (props) => {
  let scrollContainer: HTMLDivElement | undefined;
  const [canScrollLeft, setCanScrollLeft] = createSignal(false);
  const [canScrollRight, setCanScrollRight] = createSignal(true);

  const artworkQuery = useQuery(() => ({
    queryKey: ["artworks", props.category],
    queryFn: () => fetchArtworksByCategory(props.category),
  }));

  const updateScrollability = () => {
    if (!scrollContainer) return;
    const { scrollLeft, scrollWidth, clientWidth } = scrollContainer;
    setCanScrollLeft(scrollLeft > 5);
    setCanScrollRight(scrollLeft < scrollWidth - clientWidth - 5);
  };

  const scroll = (direction: "left" | "right") => {
    if (!scrollContainer) return;
    const cardWidth = scrollContainer.firstElementChild?.clientWidth ?? 0;
    const scrollAmount = cardWidth * 3 * (direction === "left" ? -1 : 1);
    scrollContainer.scrollBy({ left: scrollAmount, behavior: "smooth" });
  };

  createEffect(() => {
    if (scrollContainer) {
      updateScrollability();
      const el = scrollContainer;
      el.addEventListener("scroll", updateScrollability, { passive: true });
      // Also check on resize
      const resizeObserver = new ResizeObserver(updateScrollability);
      resizeObserver.observe(el);

      return () => {
        el.removeEventListener("scroll", updateScrollability);
        resizeObserver.unobserve(el);
      };
    }
  });

  return (
    <section class="flex items-center gap-4 md:gap-6">
      <h2 class="hidden md:block font-bold text-xl writing-mode-vertical-rl transform rotate-180 text-gray-700">
        {props.category}
      </h2>
      <div class="flex-grow relative">
        <button
          onClick={() => scroll("left")}
          disabled={!canScrollLeft()}
          class="carousel-nav-button left-0"
        >
          <CaretLeft size={24} />
        </button>
        <div
          ref={scrollContainer}
          class="flex space-x-6 overflow-x-auto snap-x snap-mandatory scrollbar-hide py-2"
        >
          <Suspense
            fallback={
              <For each={Array(5)}>{() => <ArtworkCardSkeleton />}</For>
            }
          >
            <For each={artworkQuery.data}>
              {(artwork) => <ArtworkCard artwork={artwork} />}
            </For>
          </Suspense>
        </div>
        <button
          onClick={() => scroll("right")}
          disabled={!canScrollRight()}
          class="carousel-nav-button right-0"
        >
          <CaretRight size={24} />
        </button>
      </div>
    </section>
  );
};

const ArtworkCard: Component<{ artwork: Artwork }> = (props) => {
  const handleFavoriteClick = (e: MouseEvent) => {
    e.preventDefault();
    e.stopPropagation();
    // In a real app, trigger a mutation to update favorite status
    console.log(`Toggling favorite for artwork ${props.artwork.id}`);
  };

  return (
    <Link
      to="/gallery/artworks/$artworkId"
      params={{ artworkId: props.artwork.id.toString() }}
      class="flex-shrink-0 w-52 md:w-64 snap-start group"
    >
      <div class="bg-white border rounded-lg shadow-sm overflow-hidden transition-shadow hover:shadow-xl">
        <div class="overflow-hidden">
          <img
            src={props.artwork.thumbnailUrl}
            alt={props.artwork.title}
            class="w-full h-72 object-cover transition-transform duration-300 group-hover:scale-105"
          />
          <Show when={mockUser.isLoggedIn}>
            <button
              onClick={handleFavoriteClick}
              class="absolute bottom-2 right-2 w-9 h-9 flex items-center justify-center rounded-full bg-white/80 backdrop-blur-sm shadow-md hover:bg-white z-10"
              classList={{ "text-red-500": props.artwork.isFavorite }}
              aria-label="Favorite"
            >
              <Heart
                fill={props.artwork.isFavorite ? "currentColor" : "none"}
              />
              <span class="text-sm">{props.artwork.favoriteCount}</span>
            </button>
          </Show>
        </div>
        <div class="p-4">
          <h3 class="font-bold text-md truncate" title={props.artwork.title}>
            {props.artwork.title}
          </h3>
          <p class="text-sm text-gray-500">{props.artwork.period}</p>
        </div>
      </div>
    </Link>
  );
};

const ArtworkCardSkeleton: Component = () => (
  <div class="flex-shrink-0 w-52 md:w-64 snap-start">
    <div class="bg-white border rounded-lg shadow-sm overflow-hidden">
      <div class="w-full h-72 bg-gray-200 animate-pulse" />
      <div class="p-4">
        <div class="h-5 w-3/4 bg-gray-200 rounded animate-pulse" />
        <div class="h-4 w-1/2 bg-gray-200 rounded animate-pulse mt-2" />
      </div>
    </div>
  </div>
);
