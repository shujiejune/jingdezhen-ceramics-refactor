// TypeScript type definitions (User, Artwork, Post)
export interface User {
  id: string; // UUID
  nickname: string;
  avatarUrl: string;
}

export interface Tag {
  id: number;
  name: string;
  color?: string;
}

export interface ForumPost {
  id: number;
  authorNickname: string;
  authorAvatarUrl?: string; // Optional as it's a pointer in Go
  categoryName: string;
  title: string;
  content: string;
  isPinned: boolean;
  commentCount: number;
  likeCount: number; // This is the "upvote" count from the backend
  lastActivityAt: string; // time.Time becomes a string in JSON
  createdAt: string;
  tags: Tag[];
  // User-specific fields can be added here
}

export interface Note {
  id: number;
  title: string;
  content: string; // Full Markdown content
  entityType?: "artwork" | "course_chapter";
  entityId?: number;
  parentEntityId?: number;
  entityTitle?: string; // Title of the artwork or chapter
  createdAt: string;
  updatedAt: string;
}

export type ContentBlockType = "video" | "reading" | "assignment" | "quiz";

export interface ContentBlock {
  id: number;
  type: ContentBlockType;
  title: string;
  duration?: string; // e.g., "03:52"
  isPreviewable?: boolean;
  videoUrl?: string;
}

export interface Announcement {
  id: number;
  authorNickname: string;
  authorAvatarUrl?: string;
  createdAt: string;
  content: string;
}

export interface CourseChapter {
  id: number;
  title: string;
  contentBlocks: ContentBlock[];
}

export interface Course {
  id: number;
  title: string;
  instructorName: string;
  lastUpdatedAt: string; // ISO Date String
  language: string;
  thumbnailUrl: string;
  description: string;
  chapters: CourseChapter[];
  announcements?: Announcement[];
  userNotes?: Note[];
}

export interface PortfolioWork {
  id: number;
  userId: string;
  creatorNickname: string;
  creatorAvatarUrl: string; // Added for the UI
  title: string;
  description?: string;
  isEditorsChoice: boolean;
  upvotesCount: number;
  createdAt: string;
  updatedAt: string; // ISO 8601 string
  thumbnailUrl: string;
  images?: PortfolioWorkImage[];
  tags: Tag[];
  upvotedByMe: boolean;
  savedByMe: boolean;
}

export interface PortfolioWorkImage {
  id: number;
  imageUrl: string;
  isThumbnail: boolean;
  caption: string;
  displayOrder: number;
}

/** Represents an artist, who can be a platform user or a historical figure. */
export interface Artist {
  id: number;
  name: string;
  bio?: string;
  userId?: string; // Link to a platform user account
  createdAt: string;
  updatedAt?: string;
}

/** Represents a single image associated with an artwork. */
export interface ArtworkImage {
  id: number;
  artworkId: number;
  imageUrl: string;
  isPrimary: boolean;
  caption?: string;
  displayOrder: number;
}

/** Represents a piece of ceramic art in the gallery. */
export interface Artwork {
  id: number;
  title: string;
  artistId?: number;
  artistName?: string;
  artistNameOverride?: string;
  thumbnailUrl: string;
  description?: string;
  period: string;
  dimensions?: string;
  category: string;
  isFavorite?: boolean; // User-specific
  favoriteCount: number;
  noteCount: number;
  images?: ArtworkImage[];
  tags?: Tag[];
  createdAt: string;
  updatedAt?: string;
}

export type NotificationType =
  | "comment"
  | "mention"
  | "system"
  | "favorite"
  | "new_post";

export interface Notification {
  id: number;
  actorUser?: User;
  notificationType: NotificationType;
  entityType?: "post" | "artwork" | "course";
  entityId?: number;
  entityTitle?: string; // It's helpful if the backend provides the title
  message: string; // The fallback or main message
  isRead: boolean;
  createdAt: string; // ISO Date String
}

export interface EnrolledCourse {
  id: number;
  title: string;
  thumbnailUrl: string;
  progress: number; // Percentage
}

// --- Portfolio/Work Types For Notification ---
export interface UserWork {
  id: number;
  title: string;
  thumbnailUrl: string;
  upvotesCount: number;
}

// --- Forum/Post Types For Notification ---
export interface UserPost {
  id: number;
  title: string;
  categoryName: string;
  createdAt: string;
  commentCount: number;
  likeCount: number;
}

// --- Gallery/Artwork Types For Notification ---
export interface UserFavorite {
  id: number;
  title: string;
  thumbnailUrl: string;
  artistName?: string;
  period: string;
}
