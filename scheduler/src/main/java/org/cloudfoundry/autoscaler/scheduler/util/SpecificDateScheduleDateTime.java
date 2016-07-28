package org.cloudfoundry.autoscaler.scheduler.util;

/**
 * A convenience bean to hold the schedule identifier, the schedule start date time and end date time.
 * Note: This bean is created to ease the date time validation.
 * 
 */
public class SpecificDateScheduleDateTime implements Comparable<SpecificDateScheduleDateTime> {
	// An identifier for convenience (currently number till we come up with some other mechanism to identify schedule) 
	// to know which schedule is being processed. First schedule specific/recurring starts with scheduleIdentifier 0
	private String scheduleIdentifier;
	private Long startDateTime; // In milliseconds
	private Long endDateTime; // In milliseconds

	public SpecificDateScheduleDateTime(Long startDateTime, Long endDateTime) {
		this.startDateTime = startDateTime;
		this.endDateTime = endDateTime;
	}

	public String getScheduleIdentifier() {
		return scheduleIdentifier;
	}

	public void setScheduleIdentifier(String scheduleIdentifier) {
		this.scheduleIdentifier = scheduleIdentifier;
	}

	public Long getStartDateTime() {
		return startDateTime;
	}

	public Long getEndDateTime() {
		return endDateTime;
	}

	@Override
	public int compareTo(SpecificDateScheduleDateTime compareSpecificDateScheduleDateTime) {
		if (compareSpecificDateScheduleDateTime == null)
			throw new NullPointerException("The SpecificDateScheduleDateTime object to be compared is null");

		Long thisDateTime = this.getStartDateTime();
		Long compareToDateTime = compareSpecificDateScheduleDateTime.getStartDateTime();

		if (thisDateTime == null || compareToDateTime == null)
			throw new NullPointerException("One of the date time value is null");

		return Long.compare(thisDateTime, compareToDateTime);
	}

}