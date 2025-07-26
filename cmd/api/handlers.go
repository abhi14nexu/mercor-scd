package main

import (
	"net/http"
	"strconv"

	"github.com/abhi14nexu/mercor-scd/internal/models"
	"github.com/abhi14nexu/mercor-scd/internal/scd"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
)

// getJobs returns all latest job versions with optional filtering
func getJobs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var jobs []models.Job

		query := db.Scopes(scd.Latest)

		// Optional filters
		if company := c.Query("company"); company != "" {
			query = query.Where("company_id = ?", company)
		}
		if contractor := c.Query("contractor"); contractor != "" {
			query = query.Where("contractor_id = ?", contractor)
		}
		if status := c.Query("status"); status != "" {
			query = query.Where("status = ?", status)
		}

		if err := query.Find(&jobs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": jobs, "count": len(jobs)})
	}
}

// getJob returns the latest version of a specific job by business ID
func getJob(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var job models.Job
		if err := db.Scopes(scd.Latest, scd.ByBusinessID(id)).First(&job).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": job})
	}
}

// getJobVersions returns all versions of a specific job by business ID
func getJobVersions(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var jobs []models.Job
		query := db.Scopes(scd.ByBusinessID(id), scd.OrderByVersion(false))

		if err := query.Find(&jobs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if len(jobs) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Job not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": jobs, "count": len(jobs)})
	}
}

// getPayments returns all latest payment line item versions with optional filtering
func getPayments(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var payments []models.PaymentLineItem

		query := db.Scopes(scd.Latest)

		// Optional filters
		if status := c.Query("status"); status != "" {
			query = query.Where("status = ?", status)
		}
		if contractor := c.Query("contractor"); contractor != "" {
			// Join with jobs to filter by contractor
			query = query.Joins("JOIN jobs ON payment_line_items.job_uid = jobs.uid").
				Where("jobs.contractor_id = ? AND jobs.valid_to IS NULL", contractor)
		}

		if err := query.Find(&payments).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": payments, "count": len(payments)})
	}
}

// getPayment returns the latest version of a specific payment by business ID
func getPayment(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var payment models.PaymentLineItem
		if err := db.Scopes(scd.Latest, scd.ByBusinessID(id)).First(&payment).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Payment not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": payment})
	}
}

// getPaymentVersions returns all versions of a specific payment by business ID
func getPaymentVersions(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var payments []models.PaymentLineItem
		query := db.Scopes(scd.ByBusinessID(id), scd.OrderByVersion(false))

		if err := query.Find(&payments).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if len(payments) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Payment not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": payments, "count": len(payments)})
	}
}

// getTimelogs returns all latest timelog versions with optional filtering
func getTimelogs(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		var timelogs []models.Timelog

		query := db.Scopes(scd.Latest)

		// Optional filters
		if contractor := c.Query("contractor"); contractor != "" {
			// Join with jobs to filter by contractor
			query = query.Joins("JOIN jobs ON timelogs.job_uid = jobs.uid").
				Where("jobs.contractor_id = ? AND jobs.valid_to IS NULL", contractor)
		}
		if limit := c.Query("limit"); limit != "" {
			if l, err := strconv.Atoi(limit); err == nil && l > 0 {
				query = query.Limit(l)
			}
		}

		if err := query.Find(&timelogs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": timelogs, "count": len(timelogs)})
	}
}

// getTimelog returns the latest version of a specific timelog by business ID
func getTimelog(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var timelog models.Timelog
		if err := db.Scopes(scd.Latest, scd.ByBusinessID(id)).First(&timelog).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"error": "Timelog not found"})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": timelog})
	}
}

// getTimelogVersions returns all versions of a specific timelog by business ID
func getTimelogVersions(db *gorm.DB) gin.HandlerFunc {
	return func(c *gin.Context) {
		id := c.Param("id")

		var timelogs []models.Timelog
		query := db.Scopes(scd.ByBusinessID(id), scd.OrderByVersion(false))

		if err := query.Find(&timelogs).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}

		if len(timelogs) == 0 {
			c.JSON(http.StatusNotFound, gin.H{"error": "Timelog not found"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"data": timelogs, "count": len(timelogs)})
	}
}
