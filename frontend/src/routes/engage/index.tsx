import { createFileRoute, useNavigate } from "@tanstack/solid-router";
import { For, Show, createMemo } from "solid-js";
import { PageHeader } from "~/components/layout/PageHeader";
import { activityTypes, ActivityCard } from "~/components/layout/ActivityCard";
import { Link } from "@tanstack/solid-router";
import * as Select from "@kobalte/core/select";
import { CaretDown } from "~/components/icons/Phosphor";
import { z } from "zod";
import type { Activity, ActivityType } from "~/components/layout/ActivityCard";
import type { Component } from "solid-js";
import type { SelectItemProps } from "@kobalte/core/select";

// Mock data - in a real app, this would be fetched by the loader
const allActivities: Activity[] = [
  {
    id: "1",
    slug: "annual-porcelain-festival",
    title: "Annual Porcelain Festival",
    type: "Festival",
    introduction:
      "Experience the vibrant celebration of Jingdezhen's ceramic heritage.",
    imageUrl: "https://placehold.co/600x600/fecaca/991b1b?text=Festival",
  },
  {
    id: "2",
    slug: "modern-art-fair",
    title: "Modern Art Fair",
    type: "Fair",
    introduction:
      "Discover contemporary ceramic art from emerging and established artists.",
    imageUrl: "https://placehold.co/600x600/d1fae5/065f46?text=Fair",
  },
  {
    id: "3",
    slug: "imperial-kiln-museum",
    title: "Imperial Kiln Museum",
    type: "Museum",
    introduction:
      "Explore the history of imperial porcelain production at its source.",
    imageUrl: "https://placehold.co/600x600/dbeafe/1e40af?text=Museum",
  },
  {
    id: "4",
    slug: "spring-craft-market",
    title: "Spring Craft Market",
    type: "Fair",
    introduction:
      "A bustling market showcasing the work of local artisans and craftsmen.",
    imageUrl: "https://placehold.co/600x600/d1fae5/065f46?text=Fair",
  },
  {
    id: "5",
    slug: "ancient-ceramics-exhibit",
    title: "Ancient Ceramics Exhibit",
    type: "Museum",
    introduction:
      "A journey through millennia of ceramic history and innovation.",
    imageUrl: "https://placehold.co/600x600/dbeafe/1e40af?text=Museum",
  },
  {
    id: "6",
    slug: "international-ceramic-symposium",
    title: "International Ceramic Symposium",
    type: "Festival",
    introduction:
      "A gathering of global ceramic artists for workshops and lectures.",
    imageUrl: "https://placehold.co/600x600/fecaca/991b1b?text=Symposium",
  },
];

// --- Route Definition with Search Param Validation ---

// Zod schema to validate the 'type' search parameter in the URL
const engageSearchSchema = z.object({
  type: z.enum(activityTypes).default("All").catch("All"),
});

export const Route = createFileRoute("/engage/")({
  // This function validates and parses search params from the URL
  validateSearch: (search) => engageSearchSchema.parse(search),
  component: EngagePage,
});

// --- Main Page Component ---

function EngagePage() {
  // Get the validated search params from the route
  const searchParams = Route.useSearch();
  const navigate = useNavigate({ from: Route.fullPath });

  const filteredActivities = createMemo(() => {
    const selectedType = searchParams().type;
    if (selectedType === "All") {
      return allActivities;
    }
    return allActivities.filter((activity) => activity.type === selectedType);
  });

  const handleFilterChange: (newType: ActivityType | null) => void = (
    newType,
  ) => {
    // Update the URL search params on change, which triggers a re-render
    if (newType) {
      navigate({ search: { type: newType } });
    }
  };

  return (
    <div class="bg-white">
      <PageHeader
        title="Engage"
        subtitle="Discover the vibrant cultural events, historic museums, and lively fairs that make Jingdezhen the heart of the ceramic world."
      />

      <main class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        {/* Filter Bar */}
        <div class="flex justify-between items-center mb-8">
          <p class="text-gray-600 font-medium">
            {filteredActivities().length} results
          </p>
          <ActivityFilter
            selectedValue={searchParams().type}
            onValueChange={handleFilterChange}
          />
        </div>

        {/* Activities Grid */}
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-3 gap-8">
          <For each={filteredActivities()}>
            {(activity) => <ActivityCard activity={activity} />}
          </For>
        </div>
      </main>
    </div>
  );
}

// --- Sub-Component: Filter Dropdown using Kobalte ---

interface ActivityFilterProps {
  selectedValue: ActivityType;
  onValueChange: (value: ActivityType | null) => void;
}

const ActivityFilter: Component<ActivityFilterProps> = (props) => {
  return (
    <Select.Root<ActivityType>
      value={props.selectedValue}
      onChange={props.onValueChange}
      options={[...activityTypes]}
      placeholder="Filter by type"
      itemComponent={(props: SelectItemProps<ActivityType>) => (
        <Select.Item
          item={props.item}
          class="p-2 hover:bg-gray-100 cursor-pointer"
        >
          <Select.ItemLabel>{props.item.rawValue}</Select.ItemLabel>
        </Select.Item>
      )}
    >
      <Select.Trigger class="inline-flex items-center justify-between gap-2 p-2 min-w-[150px] bg-white border border-gray-300 rounded-md">
        <Select.Value<ActivityType>>
          {(state) => state.selectedOption()}
        </Select.Value>
        <Select.Icon>
          <CaretDown />
        </Select.Icon>
      </Select.Trigger>
      <Select.Portal>
        <Select.Content class="bg-white border border-gray-200 rounded-md shadow-lg">
          <Select.Listbox />
        </Select.Content>
      </Select.Portal>
    </Select.Root>
  );
};
