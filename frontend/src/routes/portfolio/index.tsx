import { createFileRoute } from "@tanstack/solid-router";
import { useQuery } from "@tanstack/solid-query";
import { For, Show, Suspense, createSignal, createEffect } from "solid-js";
import type { Component } from "solid-js";
import * as Select from "@kobalte/core/select";
import * as Tooltip from "@kobalte/core/tooltip";
import {
  CaretDown,
  Check,
  ArrowFatUp,
  BookmarkSimple,
  CaretLeft,
  CaretRight,
  CaretLineUp,
  Funnel,
  ShareFat,
  X,
  Plus,
} from "~/components/icons/Phosphor";
import { PortfolioWork } from "~/lib/types";

// --- Type Definitions ---

type SortOption =
  | "updatedAt:desc"
  | "updatedAt:asc"
  | "upvotesCount:desc"
  | "upvotesCount:asc";

// --- Mock API Fetching ---

const mockUser = {
  isLoggedIn: true,
};

const allMockWorks: PortfolioWork[] = Array.from({ length: 35 }, (_, i) => ({
  id: i + 1,
  userId: `user-${(i % 10) + 1}`,
  creatorNickname: `Artist ${String.fromCharCode(65 + (i % 10))}`,
  creatorAvatarUrl: `https://placehold.co/40x40/e0e7ff/4338ca?text=${String.fromCharCode(65 + (i % 10))}`,
  title: `Ceramic Vessel No. ${i + 1}`,
  isEditorsChoice: i < 7, // First 7 are editor's choices
  upvotesCount: Math.floor(Math.random() * 250),
  createdAt: new Date(2025, 8, 5 - i).toISOString(),
  updatedAt: new Date(2025, 8, 5 - i).toISOString(),
  thumbnailUrl: `https://placehold.co/600x400/d1fae5/065f46?text=Work+${i + 1}`,
  tags: [
    { id: 1, name: "Glazing" },
    { id: 2, name: "Stoneware" },
    { id: 3, name: "Functional" },
  ].slice(i % 2, (i % 2) + 2),
  upvotedByMe: i % 5 === 0,
  savedByMe: i % 3 === 0,
}));

const fetchEditorsChoices = async (): Promise<PortfolioWork[]> => {
  console.log("Fetching editor's choices...");
  await new Promise((r) => setTimeout(r, 400));
  return allMockWorks.filter((work) => work.isEditorsChoice);
};

const fetchAllWorks = async (
  page: number,
  sort: SortOption,
): Promise<{ works: PortfolioWork[]; totalPages: number }> => {
  console.log(`Fetching all works: page ${page}, sort ${sort}`);
  await new Promise((r) => setTimeout(r, 600));

  const [sortBy, order] = sort.split(":");

  const sortedWorks = [...allMockWorks].sort((a, b) => {
    let valA, valB;
    if (sortBy === "updatedAt") {
      valA = new Date(a.updatedAt).getTime();
      valB = new Date(b.updatedAt).getTime();
    } else {
      valA = a.upvotesCount;
      valB = b.upvotesCount;
    }
    return order === "asc" ? valA - valB : valB - valA;
  });

  const pageSize = 12; // 3 rows of 4
  const totalPages = Math.ceil(sortedWorks.length / pageSize);
  const startIndex = (page - 1) * pageSize;
  const works = sortedWorks.slice(startIndex, startIndex + pageSize);

  return { works, totalPages };
};

// --- Route Definition ---

export const Route = createFileRoute("/portfolio/")({
  component: PortfolioPage,
});

// --- Main Page Component ---

function PortfolioPage() {
  const [currentPage, setCurrentPage] = createSignal(1);
  const [sortOption, setSortOption] =
    createSignal<SortOption>("updatedAt:desc");
  const [tags, setTags] = createSignal<string[]>([]);

  const editorsChoiceQuery = useQuery(() => ({
    queryKey: ["portfolio-editors-choice"],
    queryFn: fetchEditorsChoices,
  }));

  const allWorksQuery = useQuery(() => ({
    queryKey: ["portfolio-all", currentPage(), sortOption()],
    queryFn: () => fetchAllWorks(currentPage(), sortOption()),
    keepPreviousData: true,
  }));

  return (
    <div class="container mx-auto px-4 py-8">
      <header class="text-center mb-12">
        <h1 class="text-4xl font-bold">Portfolio</h1>
        <p class="text-lg text-gray-600 mt-2">
          Discover the exceptional talent of our community.
        </p>
      </header>

      <section class="mb-16">
        <h2 class="text-2xl font-semibold mb-6">Editor's Choices</h2>
        <Suspense
          fallback={<div class="h-80 bg-gray-200 rounded-lg animate-pulse" />}
        >
          <Show when={editorsChoiceQuery.data}>
            {(works) => <EditorsChoiceCarousel works={works()} />}
          </Show>
        </Suspense>
      </section>

      <section>
        <div class="flex justify-between items-center mb-6">
          <h2 class="text-2xl font-semibold">All Works</h2>
          <TagFilter tags={tags()} setTags={setTags} />
          <PortfolioFilter
            value={sortOption()}
            onChange={(newSort) => {
              setCurrentPage(1);
              setSortOption(newSort);
            }}
          />
          <Show when={mockUser.isLoggedIn}>
            <button class="flex items-center gap-2 px-4 py-2 bg-blue-600 text-white rounded-md hover:bg-blue-700">
              <Plus />
              Create
            </button>
          </Show>
        </div>
        <Suspense fallback={<WorkGridSkeleton />}>
          <Show when={allWorksQuery.data} keyed>
            {(data) => (
              <>
                <WorkGrid works={data.works} />
                <Pagination
                  currentPage={currentPage()}
                  totalPages={data.totalPages}
                  onPageChange={setCurrentPage}
                />
              </>
            )}
          </Show>
        </Suspense>
      </section>
      <BackToTopButton />
    </div>
  );
}

// --- Sub-Components ---

const EditorsChoiceCarousel: Component<{ works: PortfolioWork[] }> = (
  props,
) => {
  let scrollContainer: HTMLDivElement | undefined;
  const [canScrollLeft, setCanScrollLeft] = createSignal(false);
  const [canScrollRight, setCanScrollRight] = createSignal(true);

  const cardWidth = 320; // Estimated width of a card + gap

  const scroll = (direction: "left" | "right") => {
    if (!scrollContainer) return;
    const scrollAmount = cardWidth * 2 * (direction === "left" ? -1 : 1);
    scrollContainer.scrollBy({ left: scrollAmount, behavior: "smooth" });
  };

  const updateScrollability = () => {
    if (!scrollContainer) return;
    const { scrollLeft, scrollWidth, clientWidth } = scrollContainer;
    setCanScrollLeft(scrollLeft > 0);
    setCanScrollRight(scrollLeft < scrollWidth - clientWidth - 1);
  };

  createEffect(() => {
    if (scrollContainer) {
      updateScrollability();
      const el = scrollContainer;
      el.addEventListener("scroll", updateScrollability);
      return () => el.removeEventListener("scroll", updateScrollability);
    }
  });

  return (
    <div class="relative">
      <div
        ref={scrollContainer}
        class="flex space-x-6 overflow-x-auto pb-4 -mb-4 snap-x snap-mandatory scrollbar-hide"
      >
        <For each={props.works}>
          {(work) => (
            <div class="flex-shrink-0 w-72 snap-start">
              <WorkCard work={work} />
            </div>
          )}
        </For>
      </div>
      <Show when={props.works.length > 4}>
        <button
          onClick={() => scroll("left")}
          disabled={!canScrollLeft()}
          class="absolute top-1/2 -translate-y-1/2 -left-5 w-10 h-10 bg-white/80 border rounded-full shadow-md flex items-center justify-center disabled:opacity-0 transition-opacity"
        >
          <CaretLeft size={20} />
        </button>
        <button
          onClick={() => scroll("right")}
          disabled={!canScrollRight()}
          class="absolute top-1/2 -translate-y-1/2 -right-5 w-10 h-10 bg-white/80 border rounded-full shadow-md flex items-center justify-center disabled:opacity-0 transition-opacity"
        >
          <CaretRight size={20} />
        </button>
      </Show>
    </div>
  );
};

const TagFilter: Component<{
  tags: string[];
  setTags: (tags: string[]) => void;
}> = (props) => {
  const [inputValue, setInputValue] = createSignal("");

  const handleKeyDown = (event: KeyboardEvent) => {
    if (event.key === "Enter" && inputValue().trim()) {
      const newTag = inputValue().trim();
      if (!props.tags.includes(newTag)) {
        props.setTags([...props.tags, newTag]);
      }
      setInputValue("");
      event.preventDefault();
    }
  };

  const removeTag = (tagToRemove: string) => {
    props.setTags(props.tags.filter((tag) => tag !== tagToRemove));
  };

  return (
    <div class="flex items-center gap-2 border rounded-md p-1 bg-white min-w-[300px]">
      <Funnel class="text-gray-400 ml-1" />
      <div class="flex-grow flex items-center flex-wrap gap-1">
        <For each={props.tags}>
          {(tag) => (
            <span class="flex items-center gap-1 bg-blue-100 text-blue-800 px-2 py-0.5 rounded-full text-sm">
              {tag}
              <button onClick={() => removeTag(tag)}>
                <X size={12} />
              </button>
            </span>
          )}
        </For>
        <input
          type="text"
          value={inputValue()}
          onInput={(e) => setInputValue(e.currentTarget.value)}
          onKeyDown={handleKeyDown}
          placeholder="Filter by tags..."
          class="flex-grow p-1 outline-none text-sm bg-transparent"
        />
      </div>
    </div>
  );
};

const PortfolioFilter: Component<{
  value: SortOption;
  onChange: (value: SortOption) => void;
}> = (props) => {
  const options: { value: SortOption; label: string }[] = [
    { value: "updatedAt:desc", label: "Newest" },
    { value: "updatedAt:asc", label: "Oldest" },
    { value: "upvotesCount:desc", label: "Most Upvoted" },
    { value: "upvotesCount:asc", label: "Least Upvoted" },
  ];

  return (
    <Select.Root<SortOption>
      options={options.map((opt) => opt.value)}
      value={props.value}
      onChange={(value) => {
        if (value) {
          props.onChange(value);
        }
      }}
      itemComponent={(props) => (
        <Select.Item item={props.item} class="select-item">
          <Select.ItemLabel>
            {options.find((o) => o.value === props.item.rawValue)!.label}
          </Select.ItemLabel>
          <Select.ItemIndicator>
            <Check />
          </Select.ItemIndicator>
        </Select.Item>
      )}
    >
      <Select.Trigger class="select-trigger" aria-label="Sort works">
        <Select.Value<SortOption>>
          {(state) =>
            options.find((o) => o.value === state.selectedOption())!.label
          }
        </Select.Value>
        <Select.Icon>
          <CaretDown />
        </Select.Icon>
      </Select.Trigger>
      <Select.Portal>
        <Select.Content class="select-content">
          <Select.Listbox />
        </Select.Content>
      </Select.Portal>
    </Select.Root>
  );
};

const WorkGrid: Component<{ works: PortfolioWork[] }> = (props) => {
  return (
    <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-8">
      <For each={props.works}>{(work) => <WorkCard work={work} />}</For>
    </div>
  );
};

const WorkCard: Component<{ work: PortfolioWork }> = (props) => {
  const formatDate = (isoString: string) => {
    return new Date(isoString).toLocaleDateString("en-CA"); // YYYY-MM-DD
  };

  const copyLink = async (event: MouseEvent) => {
    // Prevent the Link component from navigating
    event.preventDefault();
    event.stopPropagation();

    const link = `https://your-domain.com/portfolio/works/${props.work.id}`;
    try {
      await navigator.clipboard.writeText(link);
      console.log("Link copied to clipboard!");
    } catch (err) {
      console.error("Failed to copy link: ", err);
    }
  };

  const handleButtonClick = (event: MouseEvent) => {
    // Prevent the Link component from navigating when any action button is clicked
    event.preventDefault();
    event.stopPropagation();
    // Add logic for upvoting or saving here
    console.log("Action button clicked");
  };

  return (
    <div class="bg-white border border-gray-200 rounded-lg overflow-hidden shadow-sm flex flex-col h-full">
      <img
        src={props.work.thumbnailUrl}
        alt={props.work.title}
        class="w-full h-48 object-cover"
      />
      <div class="p-4 flex flex-col flex-grow">
        <h3 class="font-bold text-lg truncate" title={props.work.title}>
          {props.work.title}
        </h3>
        <div class="flex items-center gap-2 text-sm text-gray-600 mt-2">
          <img
            src={props.work.creatorAvatarUrl}
            class="w-6 h-6 rounded-full"
            alt={props.work.creatorNickname}
          />
          <span>{props.work.creatorNickname}</span>
          <span> · </span>
          <span>{formatDate(props.work.updatedAt)}</span>
        </div>
        <div class="mt-3 space-x-2">
          <For each={props.work.tags.slice(0, 2)}>
            {(tag) => (
              <span class="px-2 py-1 text-xs bg-gray-100 rounded">
                {tag.name}
              </span>
            )}
          </For>
        </div>

        {/* Separator and Stats */}
        <div class="mt-auto pt-3 border-t border-gray-200">
          <div class="flex items-center justify-between text-gray-700">
            <button
              onClick={handleButtonClick}
              class="flex items-center gap-1.5 hover:text-red-500"
              classList={{ "text-red-500": props.work.upvotedByMe }}
            >
              <ArrowFatUp
                fill={props.work.upvotedByMe ? "currentColor" : "none"}
              />
              <span>{props.work.upvotesCount}</span>
            </button>
            <div class="h-4 w-px bg-gray-200" /> {/* Vertical Separator */}
            <button
              onClick={handleButtonClick}
              class="hover:text-blue-600"
              classList={{ "text-blue-600": props.work.savedByMe }}
            >
              <BookmarkSimple
                fill={props.work.savedByMe ? "currentColor" : "none"}
              />
            </button>
            <div class="ml-auto">
              <Tooltip.Root>
                <Tooltip.Trigger
                  as="button"
                  class="hover:text-green-600"
                  onClick={copyLink}
                >
                  <ShareFat />
                </Tooltip.Trigger>
                <Tooltip.Portal>
                  <Tooltip.Content class="tooltip-content">
                    Link Copied!
                  </Tooltip.Content>
                </Tooltip.Portal>
              </Tooltip.Root>
            </div>
          </div>
        </div>
      </div>
    </div>
  );
};

const Pagination: Component<{
  currentPage: number;
  totalPages: number;
  onPageChange: (page: number) => void;
}> = (props) => {
  return (
    <div class="flex justify-center items-center space-x-2 mt-12">
      <button
        onClick={() => props.onPageChange(props.currentPage - 1)}
        disabled={props.currentPage === 1}
        class="pagination-button"
      >
        <CaretLeft />
      </button>
      <span class="text-gray-700">
        Page {props.currentPage} of {props.totalPages}
      </span>
      <button
        onClick={() => props.onPageChange(props.currentPage + 1)}
        disabled={props.currentPage === props.totalPages}
        class="pagination-button"
      >
        <CaretRight />
      </button>
    </div>
  );
};

const BackToTopButton: Component = () => {
  const [isVisible, setIsVisible] = createSignal(false);

  const handleScroll = () => {
    setIsVisible(window.scrollY > 300);
  };

  const scrollToTop = () => {
    window.scrollTo({ top: 0, behavior: "smooth" });
  };

  createEffect(() => {
    window.addEventListener("scroll", handleScroll);
    return () => window.removeEventListener("scroll", handleScroll);
  });

  return (
    <Show when={isVisible()}>
      <button
        onClick={scrollToTop}
        class="fixed bottom-8 right-8 bg-blue-600 text-white w-12 h-12 rounded-full shadow-lg flex items-center justify-center hover:bg-blue-700 transition-opacity"
      >
        <CaretLineUp size={24} />
      </button>
    </Show>
  );
};

const WorkGridSkeleton: Component = () => {
  return (
    <div class="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 lg:grid-cols-4 gap-8">
      <For each={Array.from({ length: 8 })}>
        {() => (
          <div class="bg-white border rounded-lg shadow-sm">
            <div class="w-full h-48 bg-gray-200 animate-pulse" />
            <div class="p-4">
              <div class="h-6 w-3/4 bg-gray-200 rounded animate-pulse" />
              <div class="flex items-center gap-2 mt-2">
                <div class="w-6 h-6 rounded-full bg-gray-200 animate-pulse" />
                <div class="h-4 w-1/2 bg-gray-200 rounded animate-pulse" />
              </div>
            </div>
          </div>
        )}
      </For>
    </div>
  );
};
