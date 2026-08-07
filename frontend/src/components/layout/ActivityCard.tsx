import { Link } from "@tanstack/solid-router";
import type { Component } from "solid-js";

export const activityTypes = ["All", "Festival", "Fair", "Museum"] as const;
export type ActivityType = (typeof activityTypes)[number];

export interface Activity {
  id: string;
  slug: string;
  title: string;
  type: Exclude<ActivityType, "All">;
  introduction: string;
  imageUrl: string;
}

interface ActivityCardProps {
  activity: Activity;
}

export const ActivityCard: Component<ActivityCardProps> = (props) => {
  return (
    <div class="flex flex-col">
      <div class="aspect-square bg-gray-100 rounded-lg overflow-hidden">
        <img
          src={props.activity.imageUrl}
          alt={props.activity.title}
          class="w-full h-full object-cover"
        />
      </div>
      <h3 class="mt-4 text-xl font-semibold text-gray-800">
        {props.activity.title}
      </h3>
      <p class="mt-2 text-gray-600 flex-grow">{props.activity.introduction}</p>
      <Link
        to="/engage/$slug"
        params={{ slug: props.activity.slug }}
        class="mt-4 font-semibold text-blue-600 hover:text-blue-800"
      >
        Learn more &rarr;
      </Link>
    </div>
  );
};
