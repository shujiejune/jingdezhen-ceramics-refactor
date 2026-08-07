import { Link } from "@tanstack/solid-router";
import type { Component } from "solid-js";
import { For } from "solid-js";
import { ArrowCircleRight } from "~/components/icons/Phosphor";

// Define the shape of the data this component expects
export interface DynastyData {
  id: string;
  slug: string;
  dynasty: string;
  period: string;
  startYear: number;
  endYear: number;
  description: string;
  takeaways: string[];
  imageUrl: string;
}

interface CeramicStoryCardProps {
  data: DynastyData;
}

export const CeramicStoryCard: Component<CeramicStoryCardProps> = (props) => {
  return (
    <div class="bg-white rounded-lg shadow-xl overflow-hidden flex flex-col md:flex-row max-w-4xl mx-auto">
      {/* Left Side: Image */}
      <div class="w-full md:w-1/3 flex-shrink-0">
        <img
          src={props.data.imageUrl}
          alt={`Ceramics from the ${props.data.dynasty} dynasty`}
          class="object-cover w-full h-full"
          onError={(e) => {
            e.currentTarget.onerror = null; // Prevents infinite loop if placeholder also fails
            e.currentTarget.src =
              "https://placehold.co/400x600/E2E8F0/4A5568?text=Image";
          }}
        />
      </div>

      {/* Right Side: Text Content */}
      <div class="p-8 flex flex-col">
        {/* Header Section */}
        <div class="mb-6 grid grid-cols-2 gap-x-8 gap-y-2">
          {/* --- Row 1 --- */}
          {/* Dynasty Name (Column 1) */}
          <div class="text-sm text-gray-600">Dynasty: {props.data.dynasty}</div>
          {/* Period Name (Column 2) */}
          {/* 'self-end' aligns the text to the bottom, lining it up nicely with the large dynasty title */}
          <div class="text-sm text-gray-600 self-end">
            Period: {props.data.period}
          </div>
          {/* --- Row 2 --- */}
          {/* Start Year (Column 1) */}
          <div class="text-sm text-gray-600">Start: {props.data.startYear}</div>
          {/* End Year (Column 2) */}
          <div class="text-sm text-gray-600">End: {props.data.endYear}</div>
        </div>

        {/* Description */}
        <p class="text-gray-700 leading-relaxed mb-6">
          {props.data.description}
        </p>

        {/* Takeaways */}
        <div class="mt-auto">
          <h3 class="font-semibold text-gray-800 mb-2">Key Takeaways:</h3>
          <ul class="space-y-2 list-disc list-inside text-gray-600">
            <For each={props.data.takeaways}>{(item) => <li>{item}</li>}</For>
          </ul>
        </div>

        {/* Link to Full Article */}
        <div class="mt-8 text-right">
          <Link
            to="/ceramicstory/$slug"
            params={{ slug: props.data.slug }}
            class="inline-flex items-center font-semibold text-blue-600 hover:text-blue-800 transition-colors group"
          >
            Read Full Article
            <ArrowCircleRight
              size={20}
              class="ml-1 group-hover:translate-x-1 transition-transform"
            />
          </Link>
        </div>
      </div>
    </div>
  );
};
